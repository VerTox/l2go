package gameloop

import (
	"time"

	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// pvpFlagDuration is how long a PvP flag lasts after a PvP action (L2J
// PvPVsNormalTime default). Purple name + auto-attackable for this window.
const pvpFlagDuration = 120 * time.Second

// canAttackPlayer implements L2J Skill.checkPvpSkill (без зон): атаковать другого
// игрока атакующим действием можно, только если цель уже флагнута, является PK
// (karma>0), либо атака форсирована Ctrl. flagAttacker=true, если атакующий должен
// получить PvP-флаг (Ctrl-force по чистой цели). Caller делает проверки self/dead.
func canAttackPlayer(target *registry.PlayerWorldState, ctrl bool, now time.Time) (allowed bool, flagAttacker bool) {
	if target == nil || target.Character == nil {
		return false, false
	}
	if target.IsPvPFlagged(now) || target.Character.Karma > 0 {
		return true, false
	}
	if ctrl {
		return true, true
	}
	return false, false
}

// broadcastRelation tells nearby players (and the player itself) how to render
// this player: purple + auto-attackable while PvP-flagged or carrying karma.
func (gl *GameLoop) broadcastRelation(player *registry.PlayerWorldState) {
	if player.Character == nil {
		return
	}
	pvp := int32(0)
	if player.IsPvPFlagged(time.Now()) {
		pvp = 1
	}
	karma := int32(player.Character.Karma)
	attackable := int32(0)
	if pvp == 1 || karma > 0 {
		attackable = 1
	}
	pkt := outclient.BuildRelationChanged([]outclient.PlayerRelation{{
		ObjectID:       player.CharID,
		Relation:       outclient.RelationNone,
		AutoAttackable: attackable,
		Karma:          karma,
		PvPFlag:        pvp,
	}})
	gl.broadcastToNearby(player.Position, pkt)
	gl.sendToPlayer(player, pkt)
}

// setPvPFlag (re)arms the player's PvP flag. On a fresh flag it broadcasts the
// relation change (purple name) and refreshes the player's own UserInfo.
func (gl *GameLoop) setPvPFlag(player *registry.PlayerWorldState) {
	if player == nil || player.Character == nil {
		return
	}
	was := player.IsPvPFlagged(time.Now())
	player.PvPFlagUntil = time.Now().Add(pvpFlagDuration)
	if !was {
		gl.broadcastRelation(player)
		gl.sendUserInfo(player)
	}
}

// expirePvPFlags clears flags whose window has passed, refreshing the relation
// (name back to white) and the player's own UserInfo. Called from serviceBuffs.
func (gl *GameLoop) expirePvPFlags() {
	now := time.Now()
	for _, player := range gl.world.GetAllPlayers() {
		if player.PvPFlagUntil.IsZero() {
			continue
		}
		if !now.Before(player.PvPFlagUntil) {
			player.PvPFlagUntil = time.Time{}
			gl.broadcastRelation(player)
			gl.sendUserInfo(player)
		}
	}
}

// dealDamageToPlayer applies PvP damage to a player defender: combat stance for
// both parties (so neither can instantly relog — the existing InCombat logout/
// restart block covers "can't relog while fighting"), HP reduction, StatusUpdate
// to the victim and its targeters, and death on lethal. Mirrors NPCHitEvent's
// player-damage path (npc_combat.go). Shared with future melee PvP (l2go-npi).
func (gl *GameLoop) dealDamageToPlayer(target *registry.PlayerWorldState, attackerCharID int32, damage int) {
	if target == nil || target.Character == nil || target.Character.CurrentHP <= 0 {
		return
	}

	// Both attacker and victim enter combat stance (L2J: stance on real hit).
	gl.enterCombatStance(attackerCharID)
	gl.enterCombatStance(target.CharID)

	target.Character.CurrentHP -= float64(damage)
	if target.Character.CurrentHP < 0 {
		target.Character.CurrentHP = 0
	}

	su := outclient.BuildStatusUpdate(target.CharID, []outclient.StatusAttribute{
		{ID: outclient.StatusMaxHP, Value: int32(target.Character.MaxHP)},
		{ID: outclient.StatusCurHP, Value: int32(target.Character.CurrentHP)},
	})
	if conn := gl.connections.GetConnection(target.AccountName); conn != nil {
		_ = conn.Send(su)
	}
	gl.broadcastToTargeters(target.CharID, su)

	if target.Character.CurrentHP <= 0 {
		gl.handlePlayerDeath(target.CharID, target)
	}
}
