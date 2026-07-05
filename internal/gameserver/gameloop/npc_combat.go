package gameloop

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// NPCCombatState tracks an NPC's auto-attack state.
type NPCCombatState struct {
	IsAttacking    bool
	TargetCharID   int32
	LastAttackTime time.Time
}

// NPCNextAttackEvent schedules the next NPC auto-attack swing against a player.
type NPCNextAttackEvent struct {
	At           time.Time
	NPCObjectID  int32
	TargetCharID int32
}

func (e *NPCNextAttackEvent) ExecuteAt() time.Time { return e.At }

func (e *NPCNextAttackEvent) Execute(gl *GameLoop) {
	ncs, ok := gl.npcCombatState[e.NPCObjectID]
	if !ok || !ncs.IsAttacking || ncs.TargetCharID != e.TargetCharID {
		return // attack cancelled or retargeted
	}

	npc, exists := gl.world.GetNPC(e.NPCObjectID)
	if !exists || npc.IsDead {
		gl.stopNPCAttack(e.NPCObjectID)
		return
	}

	player, exists := gl.world.GetPlayer(e.TargetCharID)
	if !exists || player.Character == nil || player.Character.CurrentHP <= 0 {
		gl.stopNPCAttack(e.NPCObjectID)
		return
	}

	// Check range (NPC attack range + collision)
	attackRange := 40
	if npc.Template != nil && npc.Template.AttackRange > 0 {
		attackRange = npc.Template.AttackRange
	}
	attackRange += 50

	dx := npc.Position.X - player.Position.X
	dy := npc.Position.Y - player.Position.Y
	distSq := dx*dx + dy*dy
	rangeSq := attackRange * attackRange
	if distSq > rangeSq {
		gl.stopNPCAttack(e.NPCObjectID)
		return
	}

	// NPC attack speed
	pAtkSpd := 300
	if npc.Template != nil && npc.Template.PAtkSpd > 0 {
		pAtkSpd = npc.Template.PAtkSpd
	}
	timeAtkMs := calcAttackSpeed(pAtkSpd)

	// NPC attack stats
	pAtk := 10
	critRate := 4
	if npc.Template != nil {
		pAtk = int(npc.Template.PAtk)
		if pAtk < 1 {
			pAtk = 1
		}
		critRate = npc.Template.CritRate
	}

	// Player defense
	pDef := 10
	evasion := 20
	if player.Character != nil {
		computed := gl.computePlayerStats(player)
		pDef = computed.PDef
		evasion = computed.Evasion
		if pDef < 1 {
			pDef = 1
		}
	}

	// NPC accuracy (rough estimate from NPC level)
	accuracy := 35
	if npc.Template != nil {
		accuracy = 30 + npc.Template.Level
	}

	// Roll hit/damage
	miss := false
	crit := false
	var damage int32

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

	gl.prom.recordCombatAttack(attackOutcome(miss, crit))

	var flags int32
	if miss {
		flags |= outclient.AttackFlagMiss
	}
	if crit {
		flags |= outclient.AttackFlagCrit
	}

	attackPkt := outclient.BuildAttack(
		npc.ObjectID,
		e.TargetCharID,
		damage,
		flags,
		int32(npc.Position.X), int32(npc.Position.Y), int32(npc.Position.Z),
		int32(player.Position.X), int32(player.Position.Y), int32(player.Position.Z),
	)
	gl.broadcastToNearby(npc.Position, attackPkt)

	now := time.Now()
	hitDelay := time.Duration(timeAtkMs/2) * time.Millisecond

	if !miss {
		gl.events.Schedule(&NPCHitEvent{
			At:           now.Add(hitDelay),
			NPCObjectID:  e.NPCObjectID,
			TargetCharID: e.TargetCharID,
			Damage:       damage,
		})
	}

	nextSwing := time.Duration(timeAtkMs) * time.Millisecond
	gl.events.Schedule(&NPCNextAttackEvent{
		At:           now.Add(nextSwing),
		NPCObjectID:  e.NPCObjectID,
		TargetCharID: e.TargetCharID,
	})

	ncs.LastAttackTime = now
}

// NPCHitEvent applies NPC damage to a player when the attack animation lands.
type NPCHitEvent struct {
	At           time.Time
	NPCObjectID  int32
	TargetCharID int32
	Damage       int32
}

func (e *NPCHitEvent) ExecuteAt() time.Time { return e.At }

func (e *NPCHitEvent) Execute(gl *GameLoop) {
	player, exists := gl.world.GetPlayer(e.TargetCharID)
	if !exists || player.Character == nil || player.Character.CurrentHP <= 0 {
		return
	}

	// Taking damage puts the player into combat stance (L2J: stance on real hit/being
	// hit, not on the attack request). (l2go-7qv)
	gl.enterCombatStance(e.TargetCharID)

	player.Character.CurrentHP -= float64(e.Damage)
	if player.Character.CurrentHP < 0 {
		player.Character.CurrentHP = 0
	}

	su := outclient.BuildStatusUpdate(e.TargetCharID, []outclient.StatusAttribute{
		{ID: outclient.StatusMaxHP, Value: int32(player.Character.MaxHP)},
		{ID: outclient.StatusCurHP, Value: int32(player.Character.CurrentHP)},
	})
	if conn := gl.connections.GetConnection(player.AccountName); conn != nil {
		_ = conn.Send(su)
	}

	gl.broadcastToTargeters(e.TargetCharID, su)

	// Damage-received message to the victim (L2J PcStatus.reduceHp): the victim name
	// is written as plain TEXT, then the attacker NPC name and the damage. (l2go-jau)
	if e.Damage > 0 {
		if npc, ok := gl.world.GetNPC(e.NPCObjectID); ok {
			gl.sendToPlayer(player, outclient.NewSystemMessage(outclient.SysMsgC1ReceivedDamageS3FromC2).
				AddString(player.Character.Name).AddNpcName(npc.TemplateID).AddInt(e.Damage).Build())
		}
	}

	if player.Character.CurrentHP <= 0 {
		gl.handlePlayerDeath(e.TargetCharID, player)
	}
}

// handlePlayerDeath processes a player death.
func (gl *GameLoop) handlePlayerDeath(charID int32, player *registry.PlayerWorldState) {
	gl.prom.recordPlayerDeath()
	player.Character.CurrentHP = 0

	diePkt := outclient.BuildPlayerDie(charID)
	gl.broadcastToNearby(player.Position, diePkt)

	gl.stopAllNPCAttacksOnPlayer(charID)
	gl.stopAttacker(charID)

	// A dead player is no longer in combat (L2J: isInCombat resets on death). Clear the
	// flag immediately rather than waiting out the 15s stance timeout — otherwise logout
	// and restart stay blocked and the player is soft-locked until it expires. (l2go-3xh.1)
	_ = gl.world.SetPlayerCombatState(charID, false)

	log.Debug().
		Int32("char_id", charID).
		Str("name", player.Character.Name).
		Msg("Player died")
}

// startNPCAttack begins an NPC's auto-attack against a player.
func (gl *GameLoop) startNPCAttack(npcObjectID, targetCharID int32) {
	if ncs, ok := gl.npcCombatState[npcObjectID]; ok && ncs.IsAttacking && ncs.TargetCharID == targetCharID {
		return
	}

	npc, exists := gl.world.GetNPC(npcObjectID)
	if !exists || npc.IsDead {
		return
	}

	gl.npcCombatState[npcObjectID] = &NPCCombatState{
		IsAttacking:    true,
		TargetCharID:   targetCharID,
		LastAttackTime: time.Now(),
	}

	startPkt := outclient.BuildAutoAttackStart(npcObjectID)
	gl.broadcastToNearby(npc.Position, startPkt)

	delay := time.Duration(rand.Intn(500)) * time.Millisecond

	gl.events.Schedule(&NPCNextAttackEvent{
		At:           time.Now().Add(delay),
		NPCObjectID:  npcObjectID,
		TargetCharID: targetCharID,
	})
}

// stopNPCAttack stops an NPC's auto-attack.
func (gl *GameLoop) stopNPCAttack(npcObjectID int32) {
	ncs, ok := gl.npcCombatState[npcObjectID]
	if !ok {
		return
	}

	ncs.IsAttacking = false

	npc, exists := gl.world.GetNPC(npcObjectID)
	if exists {
		stopPkt := outclient.BuildAutoAttackStop(npcObjectID)
		gl.broadcastToNearby(npc.Position, stopPkt)
	}

	delete(gl.npcCombatState, npcObjectID)
}

// stopAllNPCAttacksOnPlayer stops all NPCs attacking a specific player.
func (gl *GameLoop) stopAllNPCAttacksOnPlayer(charID int32) {
	for npcObjID, ncs := range gl.npcCombatState {
		if ncs.TargetCharID == charID && ncs.IsAttacking {
			gl.stopNPCAttack(npcObjID)
		}
	}
}
