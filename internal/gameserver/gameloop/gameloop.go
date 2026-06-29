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
	tickInterval          = 100 * time.Millisecond
	commandChannelSize    = 1024
	corpseDecayDelay      = 7 * time.Second
	respawnDelay          = 60 * time.Second
	combatStanceTimeout   = 15 * time.Second
	broadcastRadius       = 2500
	regionCleanupInterval = 10 * time.Second
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
	commands      chan Command
	events        EventQueue
	world         *registry.WorldRegistry
	connections   *registry.ConnectionRegistry
	combatState    map[int32]*PlayerCombatState // charID -> auto-attack state
	npcCombatState map[int32]*NPCCombatState   // NPC objectID -> NPC auto-attack state
	npcHateLists   map[int32]*HateList          // NPC objectID -> hate
	npcSpawnInfo   map[int32]SpawnInfo           // NPC objectID -> spawn data for respawn
	activeRegions  map[string]*ActiveRegion
	interactPending map[int32]int32             // charID -> NPC objectID (active approach-to-interact)

	// Configurable server rates
	expRate float64
	spRate  float64
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
		commands:       make(chan Command, commandChannelSize),
		world:          world,
		connections:    connections,
		combatState:    make(map[int32]*PlayerCombatState),
		npcCombatState: make(map[int32]*NPCCombatState),
		npcHateLists:   make(map[int32]*HateList),
		npcSpawnInfo:   make(map[int32]SpawnInfo),
		activeRegions:  make(map[string]*ActiveRegion),
		interactPending: make(map[int32]int32),
		expRate:        expRate,
		spRate:         spRate,
	}
}

// CommandChannel returns a send-only channel for handlers to submit commands.
func (gl *GameLoop) CommandChannel() chan<- Command {
	return gl.commands
}

// RegisterSpawnInfo caches spawn data for an NPC's object ID (needed for respawn).
func (gl *GameLoop) RegisterSpawnInfo(objectID int32, info SpawnInfo) {
	gl.npcSpawnInfo[objectID] = info
}

// Run starts the game loop. It blocks until ctx is cancelled.
func (gl *GameLoop) Run(ctx context.Context) error {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	lastRegionCleanup := time.Now()

	log.Info().Msg("Game loop started")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Game loop stopping")
			return nil
		case cmd := <-gl.commands:
			gl.processCommand(cmd)
		case <-ticker.C:
			gl.tick()

			// Periodic region cleanup
			if time.Since(lastRegionCleanup) > regionCleanupInterval {
				gl.deactivateStaleRegions()
				lastRegionCleanup = time.Now()
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
	for {
		e := gl.events.Peek()
		if e == nil || e.ExecuteAt().After(now) {
			break
		}
		gl.events.PopEvent()
		e.Execute(gl)
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
	}
}

// handleAttackRequest starts auto-attack on a target.
func (gl *GameLoop) handleAttackRequest(cmd CmdAttackRequest) {
	npc, exists := gl.world.GetNPC(cmd.TargetObjectID)
	if !exists || npc.IsDead {
		return
	}

	if !npc.IsAttackable() {
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

	// Set player in combat
	gl.world.SetPlayerCombatState(cmd.AttackerCharID, true)

	// Send updated UserInfo to the player (InCombat=1 for combat stance)
	if player, exists := gl.world.GetPlayer(cmd.AttackerCharID); exists {
		userInfoData := gl.buildUserInfoForPlayer(player)
		if conn := gl.connections.GetConnection(cmd.AccountName); conn != nil {
			_ = conn.Send(userInfoData)
		}
	}

	// Broadcast AutoAttackStart
	startPkt := outclient.BuildAutoAttackStart(cmd.AttackerCharID)
	gl.broadcastToNearby(cmd.AttackerPos, startPkt)

	// Schedule first attack with a small delay to let the client approach the target
	gl.events.Schedule(&NextAttackEvent{
		At:             time.Now().Add(500 * time.Millisecond),
		AttackerCharID: cmd.AttackerCharID,
		TargetObjectID: cmd.TargetObjectID,
	})
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

	// Один MoveToPawn запускает движение клиента к NPC (стоп-дистанция 100 < триггера 150,
	// чтобы персонаж зашёл внутрь радиуса интеракции).
	if conn := gl.connections.GetConnection(cmd.AccountName); conn != nil {
		_ = conn.Send(outclient.BuildMoveToPawn(
			cmd.CharID, cmd.TargetObjectID, 100,
			player.Position.X, player.Position.Y, player.Position.Z,
			npc.Position.X, npc.Position.Y, npc.Position.Z,
		))
	}

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

// handlePlayerDisconnected cleans up combat state for a disconnected player.
func (gl *GameLoop) handlePlayerDisconnected(cmd CmdPlayerDisconnected) {
	gl.stopAttacker(cmd.CharID)

	// Stop all NPCs attacking this player
	gl.stopAllNPCAttacksOnPlayer(cmd.CharID)

	// Deactivate regions for this player
	if player, exists := gl.world.GetPlayer(cmd.CharID); exists {
		gl.deactivateRegions(player.Position.X, player.Position.Y)
	}
}

// handlePlayerEnteredWorld activates regions around the player.
func (gl *GameLoop) handlePlayerEnteredWorld(cmd CmdPlayerEnteredWorld) {
	gl.activateRegions(cmd.Position.X, cmd.Position.Y)
}

// handlePlayerMoved updates active regions when a player moves.
func (gl *GameLoop) handlePlayerMoved(cmd CmdPlayerMoved) {
	// Re-activate regions around the new position
	// (simple approach: always activate, deactivation handled by timeout)
	gl.activateRegions(cmd.Position.X, cmd.Position.Y)
}

// stopAttacker stops a player's auto-attack and cleans up combat state.
func (gl *GameLoop) stopAttacker(charID int32) {
	cs, ok := gl.combatState[charID]
	if !ok {
		return
	}

	cs.IsAutoAttacking = false

	// Send AutoAttackStop to the player
	stopPkt := outclient.BuildAutoAttackStop(charID)
	if conn := gl.connections.GetConnection(cs.AccountName); conn != nil {
		_ = conn.Send(stopPkt)
	}

	// Also broadcast to nearby players
	if player, exists := gl.world.GetPlayer(charID); exists {
		gl.broadcastToNearby(player.Position, stopPkt)
	}

	// Schedule combat stance timeout
	gl.events.Schedule(&CombatStanceTimeoutEvent{
		At:     time.Now().Add(combatStanceTimeout),
		CharID: charID,
	})
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

// stopAllAttackersOnTarget stops all players attacking a specific NPC.
func (gl *GameLoop) stopAllAttackersOnTarget(targetObjectID int32) {
	for charID, cs := range gl.combatState {
		if cs.TargetObjectID == targetObjectID && cs.IsAutoAttacking {
			cs.IsAutoAttacking = false

			stopPkt := outclient.BuildAutoAttackStop(charID)
			if conn := gl.connections.GetConnection(cs.AccountName); conn != nil {
				_ = conn.Send(stopPkt)
			}

			// Also broadcast to nearby players
			if player, exists := gl.world.GetPlayer(charID); exists {
				gl.broadcastToNearby(player.Position, stopPkt)
			}

			// Schedule combat stance timeout
			gl.events.Schedule(&CombatStanceTimeoutEvent{
				At:     time.Now().Add(combatStanceTimeout),
				CharID: charID,
			})
		}
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

// broadcastToTargeters sends a packet to all players who have the given object as their target.
func (gl *GameLoop) broadcastToTargeters(objectID int32, data []byte) {
	allPlayers := gl.world.GetAllPlayers()
	for _, p := range allPlayers {
		if p.TargetID == objectID {
			if conn := gl.connections.GetConnection(p.AccountName); conn != nil {
				_ = conn.Send(data)
			}
		}
	}
}

// computePlayerStats computes derived combat stats for a player.
func (gl *GameLoop) computePlayerStats(player *registry.PlayerWorldState) models.ComputedStats {
	char := player.Character
	baseStats := models.CharacterStats{
		STR: char.BaseSTR,
		DEX: char.BaseDEX,
		CON: char.BaseCON,
		INT: char.BaseINT,
		WIT: char.BaseWIT,
		MEN: char.BaseMEN,
	}
	combat := usecase.GetCombatBaseStatsByClass(char.ClassID)
	return models.ComputeStats(baseStats, char.Level, combat)
}

// getNpcTemplate looks up an NPC template by ID.
func (gl *GameLoop) getNpcTemplate(templateID int32) *models.NpcTemplate {
	return registry.GetNpcTemplateRegistry().Get(templateID)
}

// nextObjectID generates a new unique object ID for NPCs.
func (gl *GameLoop) nextObjectID() int32 {
	return registry.NextNPCObjectID()
}
