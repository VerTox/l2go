package gameloop

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

const (
	tickInterval        = 100 * time.Millisecond
	commandChannelSize  = 1024
	corpseDecayDelay    = 7 * time.Second
	respawnDelay        = 60 * time.Second
	combatStanceTimeout = 15 * time.Second

	// broadcastRadius is how far movement/combat packets are sent. It equals the
	// visibility forget radius so every client that has the object spawned receives them.
	broadcastRadius = registry.VisibilityForgetRadius

	regionCleanupInterval = 10 * time.Second
	autosaveInterval      = 5 * time.Minute
	regenInterval         = 3 * time.Second // L2J REGEN period
)

// SpawnInfo stores the original spawn data for respawning an NPC.
type SpawnInfo struct {
	TemplateID int32
	Position   models.Position
	Heading    int32
}

// PlayerCombatState tracks a player's auto-attack state.
type PlayerCombatState struct {
	IsAutoAttacking bool
	TargetObjectID  int32
	LastAttackTime  time.Time
	AccountName     string
	AttackSpeedMs   int
}

// GameLoop is the single-goroutine game loop that owns all mutable NPC state
// and combat logic. Client handlers send commands through the Commands channel.
type GameLoop struct {
	commands        chan Command
	events          EventQueue
	world           *registry.WorldRegistry
	connections     *registry.ConnectionRegistry
	combatState     map[int32]*PlayerCombatState // charID -> auto-attack state
	aiState         map[int32]*PlayerAIState     // charID -> current intention
	npcCombatState  map[int32]*NPCCombatState    // NPC objectID -> NPC auto-attack state
	npcHateLists    map[int32]*HateList          // NPC objectID -> hate
	npcSpawnInfo    map[int32]SpawnInfo          // NPC objectID -> spawn data for respawn
	activeRegions   map[string]*ActiveRegion
	interactPending map[int32]int32 // charID -> NPC objectID (active approach-to-interact)

	// Configurable server rates
	expRate float64
	spRate  float64

	// persistSink receives value-copy character snapshots for async persistence
	// (autosave + level-up). nil until SetPersistSink is called.
	persistSink chan<- models.Character

	// autoShotSink receives charIDs whose active auto-soulshots should be recharged
	// off the loop (the DB consume runs on the draining goroutine). nil until
	// SetAutoShotSink is called.
	autoShotSink chan<- int32

	// skillData resolves skill templates for casting (l2go-lu8). nil until
	// SetSkillData is called; casting is a no-op without it.
	skillData *registry.SkillData

	// skillLearnSink receives learned skills for async DB persistence (l2go-hv9).
	// nil until SetSkillLearnSink is called.
	skillLearnSink chan<- LearnedSkill

	// skillReuse tracks per-player skill cooldowns (charID -> skillID -> ready-at).
	// Separate from item reuse. Owned by the loop; cleared on disconnect.
	skillReuse map[int32]map[int32]time.Time

	// castSeq is a monotonic counter assigning each cast a unique id so a scheduled
	// hit event can detect it was aborted/superseded.
	castSeq int64

	// prom mirrors per-tick health samples into Prometheus collectors (l2go-5pc).
	// nil until SetPromMetrics is called; every update is nil-safe so the loop runs
	// unchanged without it.
	prom *PromMetrics

	// playerScratch is a reusable buffer for the loop's genuine whole-world sweeps
	// (advancePlayerMovement, regen — both must touch every player), so they no longer
	// allocate a fresh player map every call. Loop-goroutine only and NOT reentrant: a
	// sweep iterating this buffer must not call another routine that snapshots into
	// it. Both current users are sequential and neither nests a SnapshotPlayers call,
	// so this holds. (l2go-3rx)
	playerScratch []*registry.PlayerWorldState

	// buffedPlayers / flaggedPlayers are the small active subsets the per-second
	// sweeps actually work on, so serviceBuffs / expirePvPFlags iterate only players
	// with an active effect / PvP flag instead of scanning all N online every tick
	// (most players have neither). Loop-owned: mutated only on the loop goroutine as
	// buffs/flags are added, expire, or the player disconnects. (l2go-t2q)
	buffedPlayers  map[int32]struct{}
	flaggedPlayers map[int32]struct{}
}

// New creates a new GameLoop. expRate and spRate control experience/SP multipliers (default 1.0).
func New(world *registry.WorldRegistry, connections *registry.ConnectionRegistry, expRate, spRate float64) *GameLoop {
	if expRate <= 0 {
		expRate = 1.0
	}
	if spRate <= 0 {
		spRate = 1.0
	}
	return &GameLoop{
		commands:        make(chan Command, commandChannelSize),
		world:           world,
		connections:     connections,
		combatState:     make(map[int32]*PlayerCombatState),
		aiState:         make(map[int32]*PlayerAIState),
		npcCombatState:  make(map[int32]*NPCCombatState),
		npcHateLists:    make(map[int32]*HateList),
		npcSpawnInfo:    make(map[int32]SpawnInfo),
		activeRegions:   make(map[string]*ActiveRegion),
		interactPending: make(map[int32]int32),
		skillReuse:      make(map[int32]map[int32]time.Time),
		buffedPlayers:   make(map[int32]struct{}),
		flaggedPlayers:  make(map[int32]struct{}),
		expRate:         expRate,
		spRate:          spRate,
	}
}

// SetSkillData wires the skill template registry used for casting. Kept out of
// New() like the other optional sinks.
func (gl *GameLoop) SetSkillData(sd *registry.SkillData) { gl.skillData = sd }

// SetPromMetrics wires the Prometheus collectors the loop feeds each tick. Kept
// out of New() like the other optional sinks; nil leaves instrumentation off.
func (gl *GameLoop) SetPromMetrics(pm *PromMetrics) { gl.prom = pm }

// CommandChannel returns a send-only channel for handlers to submit commands.
func (gl *GameLoop) CommandChannel() chan<- Command {
	return gl.commands
}

// RegisterSpawnInfo caches spawn data for an NPC's object ID (needed for respawn).
func (gl *GameLoop) RegisterSpawnInfo(objectID int32, info SpawnInfo) {
	gl.npcSpawnInfo[objectID] = info
}

// RegisterWorldSpawns seeds npcSpawnInfo from the NPCs already loaded into the world,
// treating each NPC's initial position/heading as its spawn point. Must be called once
// at startup after the world is populated — otherwise npcSpawnInfo is empty and no NPC
// ever respawns (RespawnEvent logs 'spawn info not found'). (l2go-c44)
func (gl *GameLoop) RegisterWorldSpawns() {
	npcs := gl.world.GetAllNPCs()
	for _, npc := range npcs {
		gl.npcSpawnInfo[npc.ObjectID] = SpawnInfo{
			TemplateID: npc.TemplateID,
			Position:   npc.Position,
			Heading:    npc.Heading,
		}
	}
	log.Info().Int("count", len(npcs)).Msg("Registered NPC spawn info for respawn")
}

// Run starts the game loop. It blocks until ctx is cancelled.
func (gl *GameLoop) Run(ctx context.Context) error {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	lastRegionCleanup := time.Now()
	lastAutosave := time.Now()
	lastRegen := time.Now()
	lastBuffService := time.Now()

	// Tick-health instrumentation: how well the single loop goroutine keeps the
	// 100ms cadence under load (scheduling gap, work time, command backlog). Owned
	// by this goroutine; reported every tickMetricsReportInterval. (l2go-rqc)
	metrics := newTickMetrics(time.Now())
	lastTickStart := time.Now()

	log.Info().Msg("Game loop started")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Game loop stopping")
			return nil
		case cmd := <-gl.commands:
			gl.processCommand(cmd)
		case <-ticker.C:
			tickStart := time.Now()
			gap := tickStart.Sub(lastTickStart)
			lastTickStart = tickStart
			cmdDepth := len(gl.commands) // backlog seen before draining

			phaseStart := time.Now()
			gl.tick()
			gl.prom.observePhase("core", time.Since(phaseStart))

			// Periodic region cleanup
			if time.Since(lastRegionCleanup) > regionCleanupInterval {
				phaseStart = time.Now()
				gl.deactivateStaleRegions()
				gl.prom.observePhase("region_cleanup", time.Since(phaseStart))
				lastRegionCleanup = time.Now()
			}

			// Periodic autosave of online players (crash-loss safety net). Runs on
			// this goroutine so character reads are race-free; the DB write happens
			// off-loop via the persist sink.
			if time.Since(lastAutosave) > autosaveInterval {
				phaseStart = time.Now()
				gl.autosaveOnlinePlayers()
				gl.prom.observePhase("autosave", time.Since(phaseStart))
				lastAutosave = time.Now()
			}

			// Periodic HP/MP/CP regeneration for living players (l2go-nty).
			if time.Since(lastRegen) > regenInterval {
				phaseStart = time.Now()
				gl.regenPlayers()
				gl.prom.observePhase("regen", time.Since(phaseStart))
				lastRegen = time.Now()
			}

			// Buff expiry + HoT/DoT ticks (l2go-c8t).
			if time.Since(lastBuffService) > buffInterval {
				phaseStart = time.Now()
				gl.serviceBuffs()
				gl.prom.observePhase("buffs", time.Since(phaseStart))
				lastBuffService = time.Now()
			}

			// Record this tick's health and periodically report the window. work
			// covers the whole iteration (tick + periodic subsystems above) so the
			// report reflects the real per-tick budget against the 100ms deadline.
			work := time.Since(tickStart)
			metrics.record(gap, work, cmdDepth)
			// Mirror the same samples into Prometheus (nil-safe). players is set every
			// tick — cheap (one registry read) — so a scrape sees a fresh count rather
			// than the 10s window value, keeping work-vs-players correlation sharp.
			gl.prom.recordTick(gap, work, cmdDepth, gap > behindThreshold)
			gl.prom.setPlayers(gl.world.GetOnlinePlayerCount())
			if tickStart.Sub(metrics.windowStart) >= tickMetricsReportInterval {
				metrics.report(tickStart, gl.world.GetOnlinePlayerCount())
				metrics.reset(tickStart)
				// World-inventory gauges once per window: activeRegions is loop-owned
				// (read race-free here), NPC count comes from the thread-safe registry.
				activeRegions := 0
				for _, r := range gl.activeRegions {
					if r.Active {
						activeRegions++
					}
				}
				gl.prom.setWorldInventory(activeRegions, gl.world.GetNPCCount())

				// Visibility fan-out sample once per window (l2go-bws): KnownPlayers is
				// loop-owned, so this sweep on the loop goroutine is race-free. Fresh
				// snapshot (nil) to avoid touching the reusable playerScratch buffer.
				// Only KnownPlayers — KnownNPCs is owned by each connection goroutine and
				// reading it here would race.
				maxFanOut := 0
				for _, p := range gl.world.SnapshotPlayers(nil) {
					n := len(p.KnownPlayers)
					gl.prom.observeKnownPlayers(n)
					if n > maxFanOut {
						maxFanOut = n
					}
				}
				gl.prom.setKnownPlayersMax(maxFanOut)
			}
		}
	}
}

// tick executes all ready events and drains pending commands.
func (gl *GameLoop) tick() {
	// Drain all pending commands first
	for {
		select {
		case cmd := <-gl.commands:
			gl.processCommand(cmd)
		default:
			goto executeEvents
		}
	}

executeEvents:
	// Execute all events whose time has come
	now := time.Now()
	gl.advancePlayerMovement(now)
	for {
		e := gl.events.Peek()
		if e == nil || e.ExecuteAt().After(now) {
			break
		}
		gl.events.PopEvent()
		e.Execute(gl)
	}
}

// advancePlayerMovement moves every in-progress player along its path using
// server-side interpolation, so the loop and combat checks always see a fresh
// position instead of the stale value between client ValidatePosition packets.
// serverDrivenMovement reports whether the loop should interpolate this player's
// position server-side. Only combat/interact approach is server-authoritative (the
// loop needs the live distance to the target). Plain ground walking is client-
// authoritative — interpolating it here fought the client's own interpolation
// (dual authority + speed mismatch) and produced rubber-band snaps for observers. (l2go-2ax)
func serverDrivenMovement(i Intention) bool {
	return i == IntentionAttack || i == IntentionInteract
}

func (gl *GameLoop) advancePlayerMovement(now time.Time) {
	gl.playerScratch = gl.world.SnapshotPlayers(gl.playerScratch)
	for _, player := range gl.playerScratch {
		charID := player.CharID
		if !player.IsMoving {
			continue
		}
		// Skip client-authoritative ground walking (l2go-2ax): its position is synced
		// from the client's ValidatePosition packets, not interpolated here.
		if st, ok := gl.aiState[charID]; !ok || !serverDrivenMovement(st.Intention) {
			continue
		}
		speed := usecase.PlayerMoveSpeed(gl.computePlayerStats(player), player.IsRunning)
		pos, arrived := stepPlayerMovement(player, speed, now)
		_ = gl.world.UpdatePlayerPosition(context.Background(), charID, pos, player.Heading)
		// Dynamic player-to-player visibility follows the authoritative server
		// position: spawn/despawn other players as this one crosses their range.
		gl.reconcilePlayerVisibility(charID)
		if arrived {
			player.IsMoving = false
			player.MoveStartPos = models.Position{}
			player.MoveDestination = models.Position{}
			gl.onMovementArrived(charID)
		}
	}
}

// processCommand handles a single command from a client handler.
func (gl *GameLoop) processCommand(cmd Command) {
	switch c := cmd.(type) {
	case CmdAttackRequest:
		gl.handleAttackRequest(c)
	case CmdInteractRequest:
		gl.handleInteractRequest(c)
	case CmdCancelAttack:
		gl.handleCancelAttack(c)
	case CmdPlayerDisconnected:
		gl.handlePlayerDisconnected(c)
	case CmdPlayerEnteredWorld:
		gl.handlePlayerEnteredWorld(c)
	case CmdPlayerMoved:
		gl.handlePlayerMoved(c)
	case CmdMoveToLocation:
		gl.handleMoveToLocation(c)
	case CmdTeleport:
		gl.handleTeleport(c)
	case CmdRevive:
		gl.handleRevive(c)
	case CmdRestoreStats:
		gl.handleRestoreStats(c)
	case CmdCastRequest:
		gl.handleCastRequest(c)
	case CmdItemSkillCast:
		gl.handleItemSkillCast(c)
	case CmdDispel:
		gl.handleDispel(c)
	case CmdChatMessage:
		gl.handleChatMessage(c)
	case CmdOpenSkillLearn:
		gl.handleOpenSkillLearn(c)
	case CmdSkillLearnInfo:
		gl.handleSkillLearnInfo(c)
	case CmdLearnSkill:
		gl.handleLearnSkill(c)
	}
}

// handleAttackRequest starts auto-attack on a target.
func (gl *GameLoop) handleAttackRequest(cmd CmdAttackRequest) {
	tgt, exists := gl.resolveCombatTarget(cmd.TargetObjectID)
	if !exists || tgt.dead {
		return
	}

	attacker, ok := gl.world.GetPlayer(cmd.AttackerCharID)
	if !ok {
		return
	}

	if tgt.isPlayer() {
		if tgt.objectID == cmd.AttackerCharID {
			return // can't attack self
		}
		// PvP gate (L2J checkPvpSkill / onForcedAttack): plain click needs the
		// target flagged/PK; Ctrl force (Attack 0x01) always allowed but flags us.
		allowed, flagAttacker := canAttackPlayer(tgt.player, cmd.Force, time.Now())
		if !allowed {
			gl.sendToPlayer(attacker, outclient.BuildSystemMessageNoParams(outclient.SysMsgIncorrectTarget))
			if conn := gl.connections.GetConnection(cmd.AccountName); conn != nil {
				_ = conn.Send(outclient.BuildActionFailed())
			}
			return
		}
		if flagAttacker {
			gl.setPvPFlag(attacker)
		}
	} else if !tgt.npc.IsAttackable() {
		return
	}

	// Already attacking this target
	if cs, ok := gl.combatState[cmd.AttackerCharID]; ok && cs.IsAutoAttacking && cs.TargetObjectID == cmd.TargetObjectID {
		return
	}

	// Create combat state
	gl.combatState[cmd.AttackerCharID] = &PlayerCombatState{
		IsAutoAttacking: true,
		TargetObjectID:  cmd.TargetObjectID,
		LastAttackTime:  time.Now(),
		AccountName:     cmd.AccountName,
	}

	// NOTE: combat stance (AutoAttackStart + InCombat) is intentionally NOT started here.
	// Per L2J the weapon-drawn stance begins on the first real swing (doAttack →
	// clientStartAutoAttack) or on being hit, NOT on the attack request — otherwise a
	// click that never connects (target out of reach / cancelled) would draw the weapon
	// for nothing. See enterCombatStance, called from the swing in NextAttackEvent and
	// from NPCHitEvent (taking damage). (l2go-7qv)

	// Set ATTACK intention. If the target is out of reach, start SERVER-SIDE movement
	// toward it: the tick interpolates the position and fires onMovementArrived (which
	// begins the swing) on server arrival — no dependency on stale client position.
	// If already in reach, begin swinging immediately.
	gl.setIntention(cmd.AttackerCharID, IntentionAttack, cmd.TargetObjectID)
	reach := gl.meleeReachTo(attacker, tgt)
	dx := attacker.Position.X - tgt.pos.X
	dy := attacker.Position.Y - tgt.pos.Y
	if dx*dx+dy*dy > reach*reach {
		gl.startMoveToTargetPos(attacker, tgt.objectID, tgt.pos, reach)
	}
	// Always schedule the first NextAttackEvent regardless of distance. If out of
	// reach, NextAttackEvent will see distSq > rangeSq and enter the heartbeat loop
	// (re-schedule every 400 ms) until the player closes in. This ensures the chain
	// exists even when startMoveToTarget is a no-op or IsMoving is cleared off-tick.
	gl.beginAttackSwing(cmd.AttackerCharID, cmd.TargetObjectID)
}

// handleInteractRequest starts approaching a non-attackable NPC to open its
// dialogue on arrival (mirrors the attack approach via NextAttackEvent).
func (gl *GameLoop) handleInteractRequest(cmd CmdInteractRequest) {
	npc, exists := gl.world.GetNPC(cmd.TargetObjectID)
	if !exists || npc.IsDead || npc.IsAttackable() {
		return
	}
	// Дедуп: повторные клики по тому же NPC во время подхода не плодят новые цепочки
	// и НЕ ресендят MoveToPawn (иначе клиент перезапускает движение и «спотыкается»).
	if gl.interactPending[cmd.CharID] == cmd.TargetObjectID {
		return
	}
	player, exists := gl.world.GetPlayer(cmd.CharID)
	if !exists {
		return
	}
	gl.interactPending[cmd.CharID] = cmd.TargetObjectID

	// New interact intention abandons any active attack (L2J: a new intention
	// abandons the previous one), so the two heartbeats don't fight over movement.
	if cs, ok := gl.combatState[cmd.CharID]; ok && cs.IsAutoAttacking {
		gl.stopAttacker(cmd.CharID)
	}

	// INTERACT intention + server-side movement toward the NPC (the tick interpolates
	// position). InteractApproachEvent is a heartbeat that re-checks distance against
	// the SERVER position and opens the dialogue on arrival — no stale-client polling.
	gl.setIntention(cmd.CharID, IntentionInteract, cmd.TargetObjectID)
	gl.startMoveToTarget(player, npc, gl.interactApproachOffset(player, npc))

	gl.events.Schedule(&InteractApproachEvent{
		At:             time.Now().Add(300 * time.Millisecond),
		CharID:         cmd.CharID,
		TargetObjectID: cmd.TargetObjectID,
		AccountName:    cmd.AccountName,
	})
}

// handleCancelAttack stops a player's auto-attack.
func (gl *GameLoop) handleCancelAttack(cmd CmdCancelAttack) {
	gl.stopAttacker(cmd.CharID)
}

// handleMoveToLocation cancels attack/interact intention when the player issues a
// ground move, so the combat/interact heartbeat stops chasing the old target.
func (gl *GameLoop) handleMoveToLocation(cmd CmdMoveToLocation) {
	// Stop auto-attack if active (stopAttacker also clears intention).
	// setIntention(MoveTo) below overwrites intention unconditionally, so the
	// else-clearIntention branch is dead and has been removed.
	if cs, ok := gl.combatState[cmd.CharID]; ok && cs.IsAutoAttacking {
		gl.stopAttacker(cmd.CharID)
	}
	// Drop any pending interact approach.
	delete(gl.interactPending, cmd.CharID)
	// Moving interrupts an in-progress cast (L2J abortCast on move).
	if player, ok := gl.world.GetPlayer(cmd.CharID); ok {
		gl.abortCast(player)
	}
	gl.setIntention(cmd.CharID, IntentionMoveTo, 0)
}

// handlePlayerDisconnected cleans up combat state for a disconnected player.
func (gl *GameLoop) handlePlayerDisconnected(cmd CmdPlayerDisconnected) {
	gl.stopAttacker(cmd.CharID)

	// Drop skill cooldowns for the disconnected player (mirrors item-reuse cleanup).
	delete(gl.skillReuse, cmd.CharID)

	// Drop the per-tick sweep memberships so they don't leak or fire on a gone
	// player (the sweeps also self-heal a stale entry, but untrack eagerly). (l2go-t2q)
	delete(gl.buffedPlayers, cmd.CharID)
	delete(gl.flaggedPlayers, cmd.CharID)

	// Stop all NPCs attacking this player
	gl.stopAllNPCAttacksOnPlayer(cmd.CharID)

	// Despawn this player from everyone who had them in view (and clear the known
	// sets) so a later reconnect is spawned fresh.
	gl.despawnPlayerFromAll(cmd.CharID)

	// Deactivate regions for this player
	if player, exists := gl.world.GetPlayer(cmd.CharID); exists {
		gl.deactivateRegions(player.Position.X, player.Position.Y)
	}
}

// handlePlayerEnteredWorld activates regions around the player and establishes the
// initial player-to-player visibility (spawns nearby players to the newcomer and
// the newcomer to them).
func (gl *GameLoop) handlePlayerEnteredWorld(cmd CmdPlayerEnteredWorld) {
	gl.activateRegions(cmd.Position.X, cmd.Position.Y)
	gl.reconcilePlayerVisibility(cmd.CharID)
}

// handlePlayerMoved updates active regions and player-to-player visibility when a
// player moves. For client-authoritative ground walking the tick no longer
// interpolates the position (l2go-2ax), so visibility must be reconciled here off
// the client's ValidatePosition updates (the handler syncs the client position into
// the registry before sending this command, so the read below is fresh).
func (gl *GameLoop) handlePlayerMoved(cmd CmdPlayerMoved) {
	// Re-activate regions around the new position
	// (simple approach: always activate, deactivation handled by timeout)
	gl.activateRegions(cmd.Position.X, cmd.Position.Y)

	// Ground walking is client-authoritative (l2go-2ax): sync the client-reported
	// position into the registry here (loop is the single writer for it). Combat/
	// interact approach is server-interpolated in the tick, so don't clobber it.
	if st, ok := gl.aiState[cmd.CharID]; !ok || !serverDrivenMovement(st.Intention) {
		if p, exists := gl.world.GetPlayer(cmd.CharID); exists {
			_ = gl.world.UpdatePlayerPosition(context.Background(), cmd.CharID, cmd.Position, p.Heading)
		}
	}

	gl.reconcilePlayerVisibility(cmd.CharID)
}

// enterCombatStance puts a player into the weapon-drawn combat stance on the first
// real swing dealt (or on being hit). Per L2J, AutoAttackStart(objectId) is broadcast
// from doAttack()/onEvtAttacked() — NOT from the attack request — and the stance is
// sheathed only after the 15s timeout (CombatStanceTimeoutEvent), never immediately.
// The broadcast happens once per stance, guarded by the InCombat flag; that flag also
// feeds CharInfo's isInCombat byte (HP-bar/combat state seen by nearby players). The
// UserInfo InCombat field is not serialised to the owner's client, so no UserInfo
// refresh is needed here. (l2go-7qv)
func (gl *GameLoop) enterCombatStance(charID int32) {
	player, exists := gl.world.GetPlayer(charID)
	if !exists || player.InCombat {
		return
	}
	gl.world.SetPlayerCombatState(charID, true)
	gl.broadcastToNearby(player.Position, outclient.BuildAutoAttackStart(charID))
}

// stopAttacker stops a player's auto-attack and cleans up combat state.
func (gl *GameLoop) stopAttacker(charID int32) {
	cs, ok := gl.combatState[charID]
	if !ok {
		return
	}

	cs.IsAutoAttacking = false

	// Per L2J the weapon-drawn stance is NOT sheathed when auto-attack stops —
	// clientStopAutoAttack() is a no-op for players; AutoAttackStop is sent only after
	// the 15s AttackStanceTaskManager timeout (CombatStanceTimeoutEvent below).
	gl.events.Schedule(&CombatStanceTimeoutEvent{
		At:     time.Now().Add(combatStanceTimeout),
		CharID: charID,
	})

	gl.clearIntention(charID)
}

// handleNPCDeath processes an NPC death: broadcasts Die, stops all attackers, schedules respawn.
func (gl *GameLoop) handleNPCDeath(npc *models.NpcInstance) {
	npc.IsDead = true
	npc.CurrentHP = 0

	// Broadcast Die packet
	diePkt := outclient.BuildDie(npc.ObjectID)
	gl.broadcastToNearby(npc.Position, diePkt)

	// Stop all players attacking this NPC
	gl.stopAllAttackersOnTarget(npc.ObjectID)

	// Stop NPC's own auto-attack
	gl.stopNPCAttack(npc.ObjectID)

	// Award EXP/SP to attackers
	gl.awardExpForNPCKill(npc)

	now := time.Now()

	// Schedule corpse decay
	gl.events.Schedule(&CorpseDecayEvent{
		At:       now.Add(corpseDecayDelay),
		ObjectID: npc.ObjectID,
	})

	// Schedule respawn
	gl.events.Schedule(&RespawnEvent{
		At:       now.Add(respawnDelay),
		ObjectID: npc.ObjectID,
	})

	log.Debug().
		Int32("object_id", npc.ObjectID).
		Str("name", npc.Template.Name).
		Msg("NPC died")
}

// stopAllAttackersOnTarget releases every player attacking a now-dead NPC. Mirrors
// L2J's lazy onIntentionIdle on target death (thinkAttack → checkTargetLostOrDead →
// AI_INTENTION_ACTIVE/IDLE): stop the auto-attack, reset intention, halt server-side
// approach movement, and broadcast StopMove so the client leaves "follow pawn" mode
// and becomes movable again. Without StopMove the client stays locked onto the corpse
// and ignores ground-move clicks until ESC, even after the corpse decays (l2go-p80).
// The weapon-drawn stance is intentionally kept for the 15s stance timeout (retail
// behaviour), and the player's target is NOT cleared (the dead NPC stays selected).
func (gl *GameLoop) stopAllAttackersOnTarget(targetObjectID int32) {
	for charID, cs := range gl.combatState {
		if cs.TargetObjectID != targetObjectID || !cs.IsAutoAttacking {
			continue
		}
		cs.IsAutoAttacking = false
		gl.clearIntention(charID)

		// Halt server-side approach movement and tell the client to stop where it is,
		// breaking the follow-pawn lock on the dead target (the actual unblock).
		if player, exists := gl.world.GetPlayer(charID); exists {
			player.IsMoving = false
			player.MoveStartPos = models.Position{}
			player.MoveDestination = models.Position{}
			gl.broadcastToNearby(player.Position, outclient.BuildStopMove(
				charID,
				int32(player.Position.X), int32(player.Position.Y), int32(player.Position.Z),
				player.Heading,
			))
		}

		// ActionFailed to the attacker only (not broadcast): clears any "pending action"
		// left by a re-click on the target landed just before it died. Without it the
		// client waits forever for that click's confirmation and ignores the Die/StopMove
		// above, freezing until the target is cancelled (L2J onIntentionAttack
		// "else client freezes until cancel target" / L2Object.onAction). (l2go-p80)
		if conn := gl.connections.GetConnection(cs.AccountName); conn != nil {
			_ = conn.Send(outclient.BuildActionFailed())
		}

		// Sheath the weapon only after the 15s stance timeout (L2J AttackStanceTaskManager).
		gl.events.Schedule(&CombatStanceTimeoutEvent{
			At:     time.Now().Add(combatStanceTimeout),
			CharID: charID,
		})
	}
}

// broadcastToNearby sends packet data to all players within broadcast radius.
func (gl *GameLoop) broadcastToNearby(pos models.Position, data []byte) {
	players := gl.world.GetPlayersInRange(pos, broadcastRadius)
	for _, p := range players {
		if conn := gl.connections.GetConnection(p.AccountName); conn != nil {
			_ = conn.Send(data)
		}
	}
}

// broadcastToTargeters sends a packet to all players who have the given object as
// their target. Backed by the reverse targeter index (l2go-45b): O(targeters), not
// O(online) — previously this copied the whole player map and scanned it on every
// call, which, called per-player each regen/DoT tick, was O(N^2) with no crowding.
func (gl *GameLoop) broadcastToTargeters(objectID int32, data []byte) {
	for _, charID := range gl.world.GetPlayersTargeting(objectID) {
		p, ok := gl.world.GetPlayer(charID)
		if !ok {
			continue
		}
		if conn := gl.connections.GetConnection(p.AccountName); conn != nil {
			_ = conn.Send(data)
		}
	}
}

// computePlayerStats computes derived combat stats for a player, memoized on the
// player state. Called on the loop goroutine from movement/combat/cast/EXP/target;
// the cache is invalidated on any stat change (buffs, level-up, equip), so a hit is
// the common case between changes. (l2go-gur)
func (gl *GameLoop) computePlayerStats(player *registry.PlayerWorldState) models.ComputedStats {
	if s, ok := player.CachedStats(); ok {
		return s
	}
	s := usecase.ComputeCharacterStats(player.Character)
	player.SetCachedStats(s)
	return s
}

// getNpcTemplate looks up an NPC template by ID.
func (gl *GameLoop) getNpcTemplate(templateID int32) *models.NpcTemplate {
	return registry.GetNpcTemplateRegistry().Get(templateID)
}

// nextObjectID generates a new unique object ID for NPCs.
func (gl *GameLoop) nextObjectID() int32 {
	return registry.NextNPCObjectID()
}
