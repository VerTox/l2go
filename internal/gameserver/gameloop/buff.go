package gameloop

import (
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// buffInterval is how often the loop services buff expiry and HoT/DoT ticks.
const buffInterval = time.Second

// isBuffSkill reports whether a skill applies a continuous effect (buff/toggle)
// rather than an instant one.
func isBuffSkill(skill *models.Skill) bool {
	return skill.OperateType.IsContinuous() || skill.OperateType.IsSelfContinuous() || skill.IsToggle()
}

// buildBuffFromSkill builds a runtime BuffInfo from a continuous skill's GENERAL/
// SELF effects: Buff stat funcs become Mods; TickHp/TickMp/TickHpFatal become
// periodic Ticks. Returns nil if the skill declares no buff-relevant effects.
func buildBuffFromSkill(skill *models.Skill, now time.Time) *models.BuffInfo {
	b := &models.BuffInfo{
		SkillID:      int32(skill.ID),
		SkillLevel:   int32(skill.Level),
		DisplayID:    int32(skill.DisplayID),
		DisplayLevel: int32(skill.DisplayLevel),
		AbnormalType: skill.AbnormalType,
		AbnormalLvl:  skill.AbnormalLvl,
		DurationSec:  skill.AbnormalTime,
		Toggle:       skill.IsToggle(),
	}
	for _, eff := range skill.Effects {
		if eff.Scope != models.ScopeGeneral && eff.Scope != models.ScopeSelf {
			continue
		}
		switch eff.Name {
		case "Buff", "Debuff":
			b.Mods = append(b.Mods, models.ModifiersFromFuncs(eff.Funcs)...)
		case "TickHp":
			b.Ticks = append(b.Ticks, tickFromEffect(eff, models.TickHP, false))
		case "TickHpFatal":
			b.Ticks = append(b.Ticks, tickFromEffect(eff, models.TickHP, true))
		case "TickMp":
			b.Ticks = append(b.Ticks, tickFromEffect(eff, models.TickMP, false))
		}
	}
	if len(b.Mods) == 0 && len(b.Ticks) == 0 {
		return nil
	}

	if b.DurationSec > 0 && !b.Toggle {
		b.ExpiresAt = now.Add(time.Duration(b.DurationSec) * time.Second)
	}
	if b.HasTicks() {
		b.NextTick = now.Add(b.TickInterval())
	}
	return b
}

func tickFromEffect(eff models.SkillEffect, kind models.TickKind, fatal bool) models.BuffTick {
	interval := parseIntFloor(eff.Params["ticks"])
	if interval <= 0 {
		interval = 5 // L2J default tick period
	}
	return models.BuffTick{
		Kind:        kind,
		Power:       effectPower(eff),
		IntervalSec: interval,
		Fatal:       fatal,
	}
}

// applyBuff applies a continuous skill's effect to a player target, refreshing the
// buff bar and stats. No-op if the target isn't a player or the skill declares no
// buff effects. A stronger same-abnormalType buff already present blocks it.
func (gl *GameLoop) applyBuff(targetID int32, skill *models.Skill) {
	target, ok := gl.world.GetPlayer(targetID)
	if !ok || target.Character == nil {
		return
	}
	buff := buildBuffFromSkill(skill, time.Now())
	if buff == nil {
		return
	}
	if !target.Effects.Add(buff) {
		return // a stronger buff of this abnormal type is active
	}
	gl.rebuildStatMods(target)
	gl.sendAbnormalStatus(target)
	gl.sendUserInfo(target)
}

// handleDispel cancels one of a player's active buffs (RequestDispel — the player
// clicked a buff icon off). Reuses the buff-removal path.
func (gl *GameLoop) handleDispel(cmd CmdDispel) {
	player, ok := gl.world.GetPlayer(cmd.CasterCharID)
	if !ok {
		return
	}
	gl.toggleOff(player, cmd.SkillID)
}

// toggleOff removes a toggle skill from a player (recast). Returns true if it was
// active and removed.
func (gl *GameLoop) toggleOff(player *registry.PlayerWorldState, skillID int32) bool {
	if !player.Effects.RemoveSkill(skillID) {
		return false
	}
	gl.rebuildStatMods(player)
	gl.sendAbnormalStatus(player)
	gl.sendUserInfo(player)
	return true
}

// rebuildStatMods recomputes Character.StatMods (passive + equipment + buff mods).
// Called whenever the buff set changes.
func (gl *GameLoop) rebuildStatMods(player *registry.PlayerWorldState) {
	player.RebuildStatMods()
}

// sendAbnormalStatus pushes the active-buff bar (icons + timers) to the player.
func (gl *GameLoop) sendAbnormalStatus(player *registry.PlayerWorldState) {
	now := time.Now()
	buffs := make([]outclient.AbnormalBuff, 0, player.Effects.Len())
	for _, b := range player.Effects.Buffs() {
		remain := int32(-1) // infinite (toggle)
		if !b.ExpiresAt.IsZero() {
			remain = int32(b.ExpiresAt.Sub(now).Seconds())
			if remain < 0 {
				remain = 0
			}
		}
		buffs = append(buffs, outclient.AbnormalBuff{
			DisplayID:    b.DisplayID,
			DisplayLevel: b.DisplayLevel,
			RemainSec:    remain,
		})
	}
	gl.sendToPlayer(player, outclient.BuildAbnormalStatusUpdate(buffs))
}

// sendUserInfo rebuilds and sends the player's own UserInfo (stats changed).
func (gl *GameLoop) sendUserInfo(player *registry.PlayerWorldState) {
	if conn := gl.connections.GetConnection(player.AccountName); conn != nil {
		_ = conn.Send(gl.buildUserInfoForPlayer(player))
	}
}

// serviceBuffs expires timed buffs and fires HoT/DoT ticks for all players. Runs on
// the loop goroutine every buffInterval.
func (gl *GameLoop) serviceBuffs() {
	now := time.Now()
	for _, player := range gl.world.GetAllPlayers() {
		if player.Character == nil || player.Effects.Len() == 0 {
			continue
		}
		changed := false

		// HoT/DoT ticks on live players only.
		if player.Character.CurrentHP > 0 {
			for _, b := range player.Effects.Buffs() {
				if !b.HasTicks() || b.NextTick.IsZero() || now.Before(b.NextTick) {
					continue
				}
				gl.fireBuffTicks(player, b)
				b.NextTick = now.Add(b.TickInterval())
			}
		}

		// Expiry.
		if expired := player.Effects.RemoveExpired(now); len(expired) > 0 {
			changed = true
		}

		if changed {
			gl.rebuildStatMods(player)
			gl.sendAbnormalStatus(player)
			gl.sendUserInfo(player)
		}
	}
}

// fireBuffTicks applies one round of a buff's HoT/DoT ticks to a player.
func (gl *GameLoop) fireBuffTicks(player *registry.PlayerWorldState, b *models.BuffInfo) {
	var hp, mp int
	for _, tk := range b.Ticks {
		switch tk.Kind {
		case models.TickHP:
			hp += tk.Power
		case models.TickMP:
			mp += tk.Power
		}
	}
	if hp == 0 && mp == 0 {
		return
	}
	// Positive → restore; negative → damage (clamped so a DoT tick can't over-drain).
	gl.handleRestoreStats(CmdRestoreStats{CharID: player.CharID, HP: int32(maxInt(hp, 0)), MP: int32(maxInt(mp, 0))})
	if hp < 0 {
		gl.applyDoTToPlayer(player, -hp)
	}
}

// applyDoTToPlayer subtracts damage-over-time HP from a live player, clamped to 1
// (HoT/DoT ticks never kill outright in retail for TickHpFatal; kept simple here).
func (gl *GameLoop) applyDoTToPlayer(player *registry.PlayerWorldState, dmg int) {
	char := player.Character
	char.CurrentHP -= float64(dmg)
	if char.CurrentHP < 1 {
		char.CurrentHP = 1
	}
	su := outclient.BuildStatusUpdate(player.CharID, []outclient.StatusAttribute{
		{ID: outclient.StatusCurHP, Value: int32(char.CurrentHP)},
	})
	gl.sendToPlayer(player, su)
	gl.broadcastToTargeters(player.CharID, su)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
