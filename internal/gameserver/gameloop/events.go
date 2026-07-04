package gameloop

import (
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/rs/zerolog/log"
)

// interactRange — макс. дистанция открытия диалога NPC (L2J INTERACTION_DISTANCE).
const interactRange = 150

// InteractApproachEvent опрашивает позицию игрока, пока он не подойдёт к NPC,
// затем шлёт NpcHtmlMessage. Зеркалит approach-retry из NextAttackEvent.
type InteractApproachEvent struct {
	At             time.Time
	CharID         int32
	TargetObjectID int32
	AccountName    string
}

func (e *InteractApproachEvent) ExecuteAt() time.Time { return e.At }

func (e *InteractApproachEvent) Execute(gl *GameLoop) {
	// Снять отметку pending, если она всё ещё про этот NPC (не затирая более новый подход).
	clearPending := func() {
		if gl.interactPending[e.CharID] == e.TargetObjectID {
			delete(gl.interactPending, e.CharID)
		}
	}

	npc, exists := gl.world.GetNPC(e.TargetObjectID)
	if !exists || npc.IsDead {
		clearPending()
		return
	}
	player, exists := gl.world.GetPlayer(e.CharID)
	if !exists {
		clearPending()
		return
	}
	// Игрок переключил цель/снял выделение — прекращаем подход.
	if player.TargetID != e.TargetObjectID {
		clearPending()
		return
	}
	// interactPending is the authoritative liveness key for the interact approach
	// (mirrors combatState.IsAutoAttacking for the attack heartbeat). A ground move or
	// retarget clears it via handleMoveToLocation, which must stop this heartbeat.
	if gl.interactPending[e.CharID] != e.TargetObjectID {
		return
	}

	dx := player.Position.X - npc.Position.X
	dy := player.Position.Y - npc.Position.Y
	if dx*dx+dy*dy > interactRange*interactRange {
		// Out of range: drive server-side movement (tick interpolates) and re-check on
		// the next heartbeat. Distance is checked against the server position, not a
		// stale client packet. Mirrors the attack heartbeat. interactPending is the
		// cancellation guard: a move/retarget command clears it and this stops.
		if !player.IsMoving {
			gl.startMoveToTarget(player, npc, gl.interactApproachOffset(player, npc))
		}
		gl.events.Schedule(&InteractApproachEvent{
			At:             time.Now().Add(400 * time.Millisecond),
			CharID:         e.CharID,
			TargetObjectID: e.TargetObjectID,
			AccountName:    e.AccountName,
		})
		return
	}

	clearPending()

	// Подошли — открываем диалог. ВАЖНО: свежий MoveToPawn непосредственно перед
	// NpcHtmlMessage (как в близкой интеракции в target.go) — иначе клиент не
	// открывает диалог (CLAUDE.md: NPC interaction requires MoveToPawn before NpcHtmlMessage).
	log.Debug().
		Int32("char_id", e.CharID).
		Int32("npc_obj_id", e.TargetObjectID).
		Msg("interact approach: arrived, opening NPC dialogue")
	if conn := gl.connections.GetConnection(e.AccountName); conn != nil {
		_ = conn.Send(outclient.BuildMoveToPawn(
			e.CharID, e.TargetObjectID, int32(gl.interactApproachOffset(player, npc)),
			player.Position.X, player.Position.Y, player.Position.Z,
			npc.Position.X, npc.Position.Y, npc.Position.Z,
		))
		_ = conn.Send(outclient.BuildNpcHtmlMessage(e.TargetObjectID, outclient.DefaultNpcHtml))
	}
}

// NextAttackEvent schedules the next auto-attack swing.
type NextAttackEvent struct {
	At             time.Time
	AttackerCharID int32
	TargetObjectID int32
}

func (e *NextAttackEvent) ExecuteAt() time.Time { return e.At }

func (e *NextAttackEvent) Execute(gl *GameLoop) {
	cs, ok := gl.combatState[e.AttackerCharID]
	if !ok || !cs.IsAutoAttacking || cs.TargetObjectID != e.TargetObjectID {
		return // attack cancelled or retargeted
	}

	tgt, exists := gl.resolveCombatTarget(e.TargetObjectID)
	if !exists || tgt.dead {
		gl.stopAttacker(e.AttackerCharID)
		return
	}

	player, exists := gl.world.GetPlayer(e.AttackerCharID)
	if !exists {
		gl.stopAttacker(e.AttackerCharID)
		return
	}

	// Attack reach per L2J: player physical attack range + both collision radii.
	// The same value is the MoveToPawn offset, so the client stops exactly within
	// range (the old hardcoded offset 60 vs range 90 left big-collision mobs short).
	reach := gl.meleeReachTo(player, tgt)

	dx := player.Position.X - tgt.pos.X
	dy := player.Position.Y - tgt.pos.Y
	distSq := dx*dx + dy*dy
	rangeSq := reach * reach
	if distSq > rangeSq {
		// Out of reach: drive server-side movement toward the target (the tick
		// interpolates position) and re-check on the next heartbeat. This is a
		// combat heartbeat, NOT client-position polling — distance is checked
		// against the server position the tick maintains. Re-scheduling here is the
		// safety net: IsMoving can be cleared off-tick (CannotMoveAnymore /
		// StartMovement no-op), and onMovementArrived alone would then never resume.
		if !player.IsMoving {
			gl.startMoveToTargetPos(player, tgt.objectID, tgt.pos, reach)
		}
		gl.events.Schedule(&NextAttackEvent{
			At:             time.Now().Add(400 * time.Millisecond),
			AttackerCharID: e.AttackerCharID,
			TargetObjectID: e.TargetObjectID,
		})
		return
	}

	// Compute attack timing
	pAtkSpd := 300 // default
	if player.Character != nil {
		computed := gl.computePlayerStats(player)
		pAtkSpd = computed.PAtkSpd
	}
	timeAtkMs := calcAttackSpeed(pAtkSpd)

	// Roll hit/crit/damage
	miss := false
	crit := false
	var damage int32

	accuracy := 35 // default
	evasion := 20  // NPC default
	pAtk := 10     // default
	pDef := 10     // default
	critRate := 4  // default

	if player.Character != nil {
		computed := gl.computePlayerStats(player)
		accuracy = computed.Accuracy
		pAtk = computed.PAtk
		critRate = computed.CritRate
	}
	// Defender stats from the resolved target (NPC template or player computed).
	evasion = tgt.evasion
	pDef = tgt.pDef
	if pDef < 1 {
		pDef = 1
	}

	// Snapshot the soulshot charge on the equipped weapon once per swing, exactly
	// like L2J doAttack reads isChargedShot into the Attack packet up front. The
	// charge is spent below only on a landed hit (never on a miss). (l2go-77a)
	var weaponObjID, ssGrade int32
	ss := false
	if player.Character != nil {
		weaponObjID = player.Character.PaperdollObjectIDs[models.SlotRHand]
		if weaponObjID != 0 {
			shots := registry.GetChargedShotRegistry()
			ss = shots.IsCharged(weaponObjID, registry.ShotSoulshot)
			ssGrade = int32(shots.ChargedGrade(weaponObjID, registry.ShotSoulshot))
		}
	}

	if !calcHitChance(accuracy, evasion) {
		miss = true
		damage = 0
	} else {
		// Soulshot doubles pAtk before defence/crit/variance (L2J ssboost).
		damage = calcPhysDamage(soulshotPAtk(pAtk, ss), pDef)
		crit = calcCrit(critRate)
		if crit {
			damage *= 2
		}
		damage = applyVariance(damage)
		if damage < 1 {
			damage = 1
		}
	}

	// Build hit flags (L2J HF Hit.java bits).
	var flags int32
	if miss {
		flags |= outclient.AttackFlagMiss
	}
	if crit {
		flags |= outclient.AttackFlagCrit
	}
	if ss {
		// USESS bit OR'd with the weapon grade id, as L2J bakes into each Hit.
		flags |= outclient.AttackFlagSS | ssGrade
	}

	// Spend the soulshot charge once, only when the swing lands (L2J spends after
	// doAttack iff hitted; a full miss keeps the charge for the next swing).
	if ss && !miss {
		registry.GetChargedShotRegistry().SetCharged(weaponObjID, registry.ShotSoulshot, false)
	}

	// Combat system messages to the attacker (L2J L2PcInstance.sendDamageMessage):
	// miss aborts with C1_ATTACK_WENT_ASTRAY; a hit reports the optional crit line
	// then C1_DONE_S3_DAMAGE_TO_C2 with the mob name and damage. (l2go-jau)
	if player.Character != nil {
		name := player.Character.Name
		if miss {
			gl.sendToPlayer(player, outclient.NewSystemMessage(outclient.SysMsgC1AttackWentAstray).
				AddPlayerName(name).Build())
		} else {
			if crit {
				gl.sendToPlayer(player, outclient.NewSystemMessage(outclient.SysMsgC1HadCriticalHit).
					AddPlayerName(name).Build())
			}
			msg := outclient.NewSystemMessage(outclient.SysMsgC1DoneS3DamageToC2).AddPlayerName(name)
			if tgt.isPlayer() {
				msg = msg.AddPlayerName(tgt.player.Character.Name)
			} else {
				msg = msg.AddNpcName(tgt.npc.TemplateID)
			}
			gl.sendToPlayer(player, msg.AddInt(damage).Build())
		}
	}

	// First real swing draws the weapon (L2J doAttack → clientStartAutoAttack). Fires on
	// hit OR miss — the swing happens either way — and only once per stance. (l2go-7qv)
	gl.enterCombatStance(e.AttackerCharID)

	// Broadcast Attack packet (animation) immediately
	attackPkt := outclient.BuildAttack(
		player.CharID,
		e.TargetObjectID,
		damage,
		flags,
		int32(player.Position.X), int32(player.Position.Y), int32(player.Position.Z),
		int32(tgt.pos.X), int32(tgt.pos.Y), int32(tgt.pos.Z),
	)
	gl.broadcastToNearby(player.Position, attackPkt)

	// Schedule HitEvent at half attack time (damage lands mid-swing)
	now := time.Now()
	hitDelay := time.Duration(timeAtkMs/2) * time.Millisecond
	if !miss {
		gl.events.Schedule(&HitEvent{
			At:             now.Add(hitDelay),
			AttackerCharID: e.AttackerCharID,
			TargetObjectID: e.TargetObjectID,
			Damage:         damage,
		})
	}

	// Schedule next swing
	nextSwing := time.Duration(timeAtkMs) * time.Millisecond
	gl.events.Schedule(&NextAttackEvent{
		At:             now.Add(nextSwing),
		AttackerCharID: e.AttackerCharID,
		TargetObjectID: e.TargetObjectID,
	})

	cs.LastAttackTime = now
}

// HitEvent applies damage when the attack animation lands.
type HitEvent struct {
	At             time.Time
	AttackerCharID int32
	TargetObjectID int32
	Damage         int32
}

func (e *HitEvent) ExecuteAt() time.Time { return e.At }

func (e *HitEvent) Execute(gl *GameLoop) {
	tgt, exists := gl.resolveCombatTarget(e.TargetObjectID)
	if !exists || tgt.dead {
		return
	}

	if tgt.isPlayer() {
		// PvP melee: deal damage to the player defender and flag the victim
		// (retaliation is then free). The gate ran at attack initiation.
		gl.dealDamageToPlayer(tgt.player, e.AttackerCharID, int(e.Damage))
		gl.setPvPFlag(tgt.player)
		return
	}

	gl.dealDamageToNPC(tgt.npc, e.AttackerCharID, int(e.Damage))

	// Auto-soulshot: arm the next swing off-loop (recharge the weapon from the
	// player's active auto-shots). Mirrors L2J recharging at the end of onHitTimer.
	gl.maybeRechargeAutoShots(e.AttackerCharID)
}

// dealDamageToNPC applies damage from a player to an NPC: reduces HP, adds hate,
// triggers retaliation against the most-hated attacker, broadcasts the HP bar, and
// handles death. Shared by melee hits and skill casts (game loop is sole writer of
// NPC HP).
func (gl *GameLoop) dealDamageToNPC(npc *models.NpcInstance, attackerCharID int32, damage int) {
	npc.CurrentHP -= float64(damage)
	if npc.CurrentHP < 0 {
		npc.CurrentHP = 0
	}

	// Add hate. The hate list drives retaliation targeting (L2J getMostHated): an NPC
	// fights the top-hate attacker, not merely whoever hit it last.
	hl, ok := gl.npcHateLists[npc.ObjectID]
	if !ok {
		hl = NewHateList()
		gl.npcHateLists[npc.ObjectID] = hl
	}
	hl.AddHate(attackerCharID, int64(damage))

	if npc.IsAttackable() {
		target := hl.GetTopAttacker()
		if target == 0 {
			target = attackerCharID
		}
		gl.startNPCAttack(npc.ObjectID, target)
	}

	su := outclient.BuildStatusUpdate(npc.ObjectID, []outclient.StatusAttribute{
		{ID: outclient.StatusMaxHP, Value: int32(npc.Template.HP)},
		{ID: outclient.StatusCurHP, Value: int32(npc.CurrentHP)},
	})
	gl.broadcastToTargeters(npc.ObjectID, su)

	if npc.CurrentHP <= 0 {
		gl.handleNPCDeath(npc)
	}
}

// RespawnEvent respawns an NPC after death.
type RespawnEvent struct {
	At       time.Time
	ObjectID int32 // old object ID (for spawn info lookup)
}

func (e *RespawnEvent) ExecuteAt() time.Time { return e.At }

func (e *RespawnEvent) Execute(gl *GameLoop) {
	info, ok := gl.npcSpawnInfo[e.ObjectID]
	if !ok {
		log.Warn().Int32("object_id", e.ObjectID).Msg("respawn: spawn info not found")
		return
	}

	tpl := gl.getNpcTemplate(info.TemplateID)
	if tpl == nil {
		log.Warn().Int32("template_id", info.TemplateID).Msg("respawn: template not found")
		return
	}

	// Create new NPC instance with new ObjectID
	newNPC := &models.NpcInstance{
		ObjectID:   gl.nextObjectID(),
		TemplateID: info.TemplateID,
		Template:   tpl,
		Position:   info.Position,
		Heading:    info.Heading,
		IsRunning:  true,
		IsDead:     false,
		CurrentHP:  tpl.HP,
		CurrentMP:  tpl.MP,
	}

	// Move the spawn info to the new object ID (drop the dead one's entry so
	// npcSpawnInfo doesn't leak an entry per respawn).
	delete(gl.npcSpawnInfo, e.ObjectID)
	gl.npcSpawnInfo[newNPC.ObjectID] = info

	// Add to world
	gl.world.AddNPC(newNPC)

	log.Debug().
		Int32("new_object_id", newNPC.ObjectID).
		Int32("template_id", info.TemplateID).
		Msg("NPC respawned")
}

// CorpseDecayEvent removes an NPC corpse from the world.
type CorpseDecayEvent struct {
	At       time.Time
	ObjectID int32
}

func (e *CorpseDecayEvent) ExecuteAt() time.Time { return e.At }

func (e *CorpseDecayEvent) Execute(gl *GameLoop) {
	// Broadcast DeleteObject to nearby players
	npc, exists := gl.world.GetNPC(e.ObjectID)
	if !exists {
		return
	}

	deleteData := outclient.BuildDeleteObject(e.ObjectID)
	gl.broadcastToNearby(npc.Position, deleteData)

	// Remove from world registry
	gl.world.RemoveNPC(e.ObjectID)

	// Clean up hate list
	delete(gl.npcHateLists, e.ObjectID)

	log.Debug().Int32("object_id", e.ObjectID).Msg("NPC corpse decayed")
}

// CombatStanceTimeoutEvent ends combat stance after 15 seconds of inactivity.
type CombatStanceTimeoutEvent struct {
	At     time.Time
	CharID int32
}

func (e *CombatStanceTimeoutEvent) ExecuteAt() time.Time { return e.At }

func (e *CombatStanceTimeoutEvent) Execute(gl *GameLoop) {
	cs, ok := gl.combatState[e.CharID]
	if !ok {
		return
	}

	// Only timeout if not actively attacking and enough time has passed
	if cs.IsAutoAttacking {
		return // Still attacking, don't timeout
	}

	if time.Since(cs.LastAttackTime) < 15*time.Second {
		return // Recent attack, reschedule
	}

	// End combat stance
	gl.world.SetPlayerCombatState(e.CharID, false)

	// Broadcast AutoAttackStop to the owner AND every nearby player so the weapon-drawn
	// stance is sheathed on EVERYONE's client — symmetric to enterCombatStance, which
	// broadcasts AutoAttackStart via broadcastToNearby. Without this, the set was broadcast
	// but the clear reached only the owner, so neighbours kept seeing the stale combat
	// stance until their own timeout. Matches L2J AttackStanceTaskManager, which does
	// actor.broadcastPacket(new AutoAttackStop(objectId)). (l2go-k0f)
	stopPkt := outclient.BuildAutoAttackStop(e.CharID)
	if player, exists := gl.world.GetPlayer(e.CharID); exists {
		gl.broadcastToNearby(player.Position, stopPkt)

		// Refresh the owner's own UserInfo (InCombat=0 for peaceful stance). This stays
		// owner-only: UserInfo is a self-packet, not part of the knownlist broadcast.
		userInfoData := gl.buildUserInfoForPlayer(player)
		if conn := gl.connections.GetConnection(cs.AccountName); conn != nil {
			_ = conn.Send(userInfoData)
		}
	} else {
		// Player already left the world: no position to broadcast from, so fall back to a
		// direct send to the owner if the connection is still around.
		if conn := gl.connections.GetConnection(cs.AccountName); conn != nil {
			_ = conn.Send(stopPkt)
		}
	}

	delete(gl.combatState, e.CharID)
}
