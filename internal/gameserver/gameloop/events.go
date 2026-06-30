package gameloop

import (
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
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

	npc, exists := gl.world.GetNPC(e.TargetObjectID)
	if !exists || npc.IsDead {
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
	reach := gl.meleeReach(player, npc)

	dx := player.Position.X - npc.Position.X
	dy := player.Position.Y - npc.Position.Y
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
			gl.startMoveToTarget(player, npc, reach)
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
	if npc.Template != nil {
		evasion = int(npc.Template.PDef / 10) // rough evasion estimate
		pDef = int(npc.Template.PDef)
		if pDef < 1 {
			pDef = 1
		}
	}

	if !calcHitChance(accuracy, evasion) {
		miss = true
		damage = 0
	} else {
		damage = calcPhysDamage(pAtk, pDef)
		crit = calcCrit(critRate)
		if crit {
			damage *= 2
		}
		damage = applyVariance(damage)
		if damage < 1 {
			damage = 1
		}
	}

	// Build flags
	var flags int32
	if miss {
		flags |= 0x01 // MISS
	}
	if crit {
		flags |= 0x20 // CRIT
	}

	// Broadcast Attack packet (animation) immediately
	attackPkt := outclient.BuildAttack(
		player.CharID,
		e.TargetObjectID,
		damage,
		flags,
		int32(player.Position.X), int32(player.Position.Y), int32(player.Position.Z),
		int32(npc.Position.X), int32(npc.Position.Y), int32(npc.Position.Z),
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
	npc, exists := gl.world.GetNPC(e.TargetObjectID)
	if !exists || npc.IsDead {
		return
	}

	// Apply damage (game loop is sole writer of NPC HP)
	npc.CurrentHP -= float64(e.Damage)
	if npc.CurrentHP < 0 {
		npc.CurrentHP = 0
	}

	// Add hate
	if hl, ok := gl.npcHateLists[e.TargetObjectID]; ok {
		hl.AddHate(e.AttackerCharID, int64(e.Damage))
	} else {
		hl := NewHateList()
		hl.AddHate(e.AttackerCharID, int64(e.Damage))
		gl.npcHateLists[e.TargetObjectID] = hl
	}

	// Trigger NPC auto-attack back (NPC retaliates when hit)
	if npc.IsAttackable() {
		gl.startNPCAttack(e.TargetObjectID, e.AttackerCharID)
	}

	// Broadcast StatusUpdate with current HP
	su := outclient.BuildStatusUpdate(e.TargetObjectID, []outclient.StatusAttribute{
		{ID: outclient.StatusMaxHP, Value: int32(npc.Template.HP)},
		{ID: outclient.StatusCurHP, Value: int32(npc.CurrentHP)},
	})
	gl.broadcastToTargeters(e.TargetObjectID, su)

	// Check death
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

	// Cache spawn info for the new object ID
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

	// Send AutoAttackStop to the player
	stopPkt := outclient.BuildAutoAttackStop(e.CharID)
	if conn := gl.connections.GetConnection(cs.AccountName); conn != nil {
		_ = conn.Send(stopPkt)
	}

	// Send updated UserInfo to the player (InCombat=0 for peaceful stance)
	if player, exists := gl.world.GetPlayer(e.CharID); exists {
		userInfoData := gl.buildUserInfoForPlayer(player)
		if conn := gl.connections.GetConnection(cs.AccountName); conn != nil {
			_ = conn.Send(userInfoData)
		}
	}

	delete(gl.combatState, e.CharID)
}
