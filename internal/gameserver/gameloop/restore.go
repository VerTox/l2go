package gameloop

import (
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

// handleRestoreStats applies a HP/MP/CP restore to a live player (clamped to maxima)
// and pushes the updated bars to the client and anyone targeting the player.
//
// INTERIM (l2go-diu): this is the landing point for potion effects until a real
// skill engine exists. It only restores vitals; it never revives a dead player.
func (gl *GameLoop) handleRestoreStats(cmd CmdRestoreStats) {
	player, exists := gl.world.GetPlayer(cmd.CharID)
	if !exists || player.Character == nil {
		return
	}
	char := player.Character
	if char.CurrentHP <= 0 {
		return // dead players are not healed by potions
	}

	if cmd.HP > 0 {
		char.CurrentHP = clampFloat(char.CurrentHP+float64(cmd.HP), float64(char.MaxHP))
	}
	if cmd.MP > 0 {
		char.CurrentMP = clampFloat(char.CurrentMP+float64(cmd.MP), float64(char.MaxMP))
	}
	if cmd.CP > 0 {
		char.CurrentCP = clampInt(char.CurrentCP+int(cmd.CP), char.MaxCP)
	}

	// HP/MP/CP bars: sent to the player and to anyone currently targeting it.
	su := outclient.BuildStatusUpdate(cmd.CharID, []outclient.StatusAttribute{
		{ID: outclient.StatusCurHP, Value: int32(char.CurrentHP)},
		{ID: outclient.StatusMaxHP, Value: int32(char.MaxHP)},
		{ID: outclient.StatusCurMP, Value: int32(char.CurrentMP)},
		{ID: outclient.StatusMaxMP, Value: int32(char.MaxMP)},
		{ID: outclient.StatusCurCP, Value: int32(char.CurrentCP)},
		{ID: outclient.StatusMaxCP, Value: int32(char.MaxCP)},
	})
	if conn := gl.connections.GetConnection(player.AccountName); conn != nil {
		_ = conn.Send(su)
		// UserInfo keeps the player's own frames (HP/MP/CP fields) authoritative.
		_ = conn.Send(gl.buildUserInfoForPlayer(player))
	}
	gl.broadcastToTargeters(cmd.CharID, su)
}

// handleItemSkillCast casts an item's linked skill (potion/consumable) through the
// real skill engine, bypassing the KnownSkills gate (item skills are not in the
// player's skill tree). Consumables have immediate effect, so this is an instant
// cast: broadcast the animation, then apply effects now — no cast bar. Routes
// restore/buff effects through applySkillEffects (l2go-849, replaces the interim
// direct-restore path). (l2go-849)
func (gl *GameLoop) handleItemSkillCast(cmd CmdItemSkillCast) {
	if gl.skillData == nil {
		return
	}
	caster, ok := gl.world.GetPlayer(cmd.CharID)
	if !ok || caster.Character == nil || caster.Character.CurrentHP <= 0 {
		return
	}
	skill := gl.skillData.GetSkill(int(cmd.SkillID), int(cmd.Level))
	if skill == nil {
		return
	}

	// Self-cast animation (caster == target; player object id == char id) so the
	// client plays the potion effect and runs the item-icon reuse sweep.
	px, py, pz := int32(caster.Position.X), int32(caster.Position.Y), int32(caster.Position.Z)
	gl.broadcastToNearby(caster.Position, outclient.BuildMagicSkillUse(
		cmd.CharID, cmd.CharID, cmd.SkillID, cmd.Level, 0, 0, px, py, pz, px, py, pz,
	))
	gl.broadcastToNearby(caster.Position, outclient.BuildMagicSkillLaunched(
		cmd.CharID, cmd.SkillID, cmd.Level, []int32{cmd.CharID},
	))
	gl.applySkillEffects(caster, cmd.CharID, skill)
}

// itemSkillCaster adapts the game loop's command channel to usecase.ItemSkillCaster
// so item handlers can trigger a real item-skill cast without depending on the loop
// internals (the loop owns casting/vitals state).
type itemSkillCaster struct {
	ch chan<- Command
}

func (c itemSkillCaster) CastItemSkill(charID, skillID, level int32) {
	c.ch <- CmdItemSkillCast{CharID: charID, SkillID: skillID, Level: level}
}

// ItemSkillCaster returns a usecase.ItemSkillCaster backed by this loop's command channel.
func (gl *GameLoop) ItemSkillCaster() usecase.ItemSkillCaster {
	return itemSkillCaster{ch: gl.commands}
}

// ensure the adapter satisfies the usecase contract at compile time.
var _ usecase.ItemSkillCaster = itemSkillCaster{}

func clampFloat(v, max float64) float64 {
	if v > max {
		return max
	}
	return v
}

func clampInt(v, max int) int {
	if v > max {
		return max
	}
	return v
}
