package gameloop

import (
	"math"
	"strconv"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// castLaunchOffset is how long before the hit the MagicSkillLaunched packet fires
// (L2J subtracts ~400ms from hitTime for the launch). We keep the launch and hit at
// the same instant for simplicity — the launch packet is sent just before effects.
const minCastTime = 500 * time.Millisecond

// handleCastRequest validates and begins a skill cast (RequestMagicSkillUse). The
// full L2J chain is collapsed to: validate → beginCast (consume MP, animation,
// gauge, schedule hit) → CastHitEvent (effects, reuse). Casting is a no-op if the
// skill registry isn't wired.
func (gl *GameLoop) handleCastRequest(cmd CmdCastRequest) {
	if gl.skillData == nil {
		return
	}
	caster, ok := gl.world.GetPlayer(cmd.CasterCharID)
	if !ok || caster.Character == nil {
		return
	}
	char := caster.Character

	if char.CurrentHP <= 0 {
		return // dead can't cast
	}
	if caster.Casting != nil {
		return // already casting (L2J isCastingNow)
	}

	level := int(caster.KnownSkills[cmd.SkillID])
	if level <= 0 {
		return // skill not known
	}
	skill := gl.skillData.GetSkill(int(cmd.SkillID), level)
	if skill == nil || (!skill.OperateType.IsActive() && !skill.IsToggle()) {
		return // unknown or non-castable (passive skills aren't cast)
	}

	// Toggle already active → recast turns it off instantly (no cast bar).
	if skill.IsToggle() && caster.Effects.HasSkill(cmd.SkillID) {
		gl.toggleOff(caster, cmd.SkillID)
		return
	}

	// Skill on cooldown?
	if gl.isSkillOnReuse(cmd.CasterCharID, cmd.SkillID) {
		return
	}

	// Resolve the target for the cast.
	target := gl.resolveCastTarget(caster, skill)
	if target == 0 {
		return // nothing valid to cast on
	}

	// Range check: the target must be within the skill's cast range (L2J validates
	// this before casting; out of range simply fails here rather than approaching).
	if !gl.targetInCastRange(caster, target, skill) {
		return
	}

	// PvP gate: an offensive skill on another player must pass checkPvpSkill
	// (flag/karma/ctrl). Blocked → INCORRECT_TARGET + ActionFailed, no cast.
	if isOffensiveSkill(skill) && target != caster.CharID {
		if tgt, isPlayer := gl.world.GetPlayer(target); isPlayer {
			allowed, flagAttacker := canAttackPlayer(tgt, cmd.CtrlPressed, time.Now())
			if !allowed {
				gl.sendToPlayer(caster, outclient.BuildSystemMessageNoParams(outclient.SysMsgIncorrectTarget))
				if conn := gl.connections.GetConnection(caster.AccountName); conn != nil {
					_ = conn.Send(outclient.BuildActionFailed())
				}
				return
			}
			if flagAttacker {
				gl.setPvPFlag(caster)
			}
		}
	}

	// MP check: L2J validates the FULL cost (mpConsume1 + mpConsume2) up front, not
	// just the initial part — otherwise a cast begins and fizzles at the hit.
	if int(char.CurrentMP) < skill.MpConsume1+skill.MpConsume2 {
		gl.sendToPlayer(caster, outclient.BuildSystemMessageNoParams(outclient.SysMsgNotEnoughMp))
		return
	}
	if skill.MpConsume1 > 0 {
		char.CurrentMP -= float64(skill.MpConsume1)
	}

	reuse := int32(skill.ReuseDelay)
	tpos := gl.objectPosition(target, caster.Position)

	// Toggles apply instantly — no cast bar (SetupGauge). Send the animation with
	// zero hit time, apply the effect immediately, and arm the reuse.
	if skill.IsToggle() {
		msu := outclient.BuildMagicSkillUse(cmd.CasterCharID, target, cmd.SkillID, int32(level), 0, reuse,
			int32(caster.Position.X), int32(caster.Position.Y), int32(caster.Position.Z),
			int32(tpos.X), int32(tpos.Y), int32(tpos.Z))
		gl.broadcastToNearby(caster.Position, msu)
		gl.broadcastToNearby(caster.Position, outclient.BuildMagicSkillLaunched(cmd.CasterCharID, cmd.SkillID, int32(level), []int32{target}))
		gl.applySkillEffects(caster, target, skill)
		gl.armSkillReuse(cmd.CasterCharID, cmd.SkillID, int32(level), skill.ReuseDelay)
		return
	}

	hitTime := gl.castTime(caster, skill)

	// Assign a unique id so a stale hit event (aborted/superseded) is ignored.
	gl.castSeq++
	caster.Casting = &registry.CastState{
		ID:         gl.castSeq,
		SkillID:    cmd.SkillID,
		SkillLevel: int32(level),
		TargetID:   target,
	}

	// Cast animation + progress gauge to everyone nearby. The target location must be
	// the target's real position (not the caster's), or the client snaps the mob to
	// the caster on a ranged cast.
	msu := outclient.BuildMagicSkillUse(cmd.CasterCharID, target, cmd.SkillID, int32(level),
		int32(hitTime.Milliseconds()), reuse,
		int32(caster.Position.X), int32(caster.Position.Y), int32(caster.Position.Z),
		int32(tpos.X), int32(tpos.Y), int32(tpos.Z))
	gl.broadcastToNearby(caster.Position, msu)
	if conn := gl.connections.GetConnection(caster.AccountName); conn != nil {
		_ = conn.Send(outclient.BuildSetupGauge(cmd.CasterCharID, outclient.GaugeColorBlue, int32(hitTime.Milliseconds())))
	}

	gl.events.Schedule(&CastHitEvent{
		At:     time.Now().Add(hitTime),
		CharID: cmd.CasterCharID,
		CastID: caster.Casting.ID,
	})
}

// objectPosition returns the world position of a target object id (player or NPC),
// falling back to def when the object isn't found.
func (gl *GameLoop) objectPosition(objectID int32, def models.Position) models.Position {
	if p, ok := gl.world.GetPlayer(objectID); ok {
		return p.Position
	}
	if npc, ok := gl.world.GetNPC(objectID); ok {
		return npc.Position
	}
	return def
}

// castTime returns the cast duration scaled by the caster's casting speed, mirroring
// L2J Formulas.calcCastTime: (hitTime / speed) * 333, where speed is MAtkSpd for
// magic skills and PAtkSpd for physical ones. Clamped to 500ms when hitTime > 500.
func (gl *GameLoop) castTime(caster *registry.PlayerWorldState, skill *models.Skill) time.Duration {
	if skill.HitTime <= 0 {
		return 0
	}
	stats := gl.computePlayerStats(caster)
	speed := stats.MAtkSpd
	if !skill.IsMagic() {
		speed = stats.PAtkSpd
	}
	if speed < 1 {
		speed = 1
	}
	ms := (float64(skill.HitTime) / float64(speed)) * 333.0
	if ms < 500 && skill.HitTime > 500 {
		ms = 500
	}
	return time.Duration(ms) * time.Millisecond
}

// targetInCastRange reports whether the target is within the skill's cast range.
// castRange <= 0 (self/unbounded) always passes; the NPC/player collision radius is
// folded in as a small margin.
func (gl *GameLoop) targetInCastRange(caster *registry.PlayerWorldState, targetID int32, skill *models.Skill) bool {
	if skill.CastRange <= 0 || targetID == caster.CharID {
		return true
	}
	tpos := gl.objectPosition(targetID, caster.Position)
	dx := caster.Position.X - tpos.X
	dy := caster.Position.Y - tpos.Y
	reach := skill.CastRange + 80 // collision/leeway margin
	return dx*dx+dy*dy <= reach*reach
}

// resolveCastTarget picks the object the cast applies to. SELF-target skills always
// hit the caster; otherwise the caster's current target, falling back to self for
// skills that can self-target. Returns 0 if nothing valid.
func (gl *GameLoop) resolveCastTarget(caster *registry.PlayerWorldState, skill *models.Skill) int32 {
	if skill.TargetType == models.TargetSelf {
		return caster.CharID
	}
	if caster.TargetID != 0 {
		return caster.TargetID
	}
	// No explicit target: beneficial skills default to self.
	if !isOffensiveSkill(skill) {
		return caster.CharID
	}
	return 0
}

// abortCast interrupts an in-progress cast (movement, damage, stun). The scheduled
// CastHitEvent becomes a no-op because Casting is cleared (its ID no longer matches).
// No-op if the player isn't casting.
func (gl *GameLoop) abortCast(player *registry.PlayerWorldState) {
	if player == nil || player.Casting == nil {
		return
	}
	player.Casting = nil
	gl.broadcastToNearby(player.Position, outclient.BuildMagicSkillCanceled(player.CharID))
}

// CastHitEvent fires at the end of the cast: applies effects and arms the cooldown.
type CastHitEvent struct {
	At     time.Time
	CharID int32
	CastID int64
}

func (e *CastHitEvent) ExecuteAt() time.Time { return e.At }

func (e *CastHitEvent) Execute(gl *GameLoop) {
	caster, ok := gl.world.GetPlayer(e.CharID)
	if !ok || caster.Casting == nil || caster.Casting.ID != e.CastID {
		return // aborted or superseded
	}
	cast := caster.Casting
	caster.Casting = nil // cast completes now

	char := caster.Character
	if char == nil || char.CurrentHP <= 0 {
		return
	}

	skill := gl.skillData.GetSkill(int(cast.SkillID), int(cast.SkillLevel))
	if skill == nil {
		return
	}

	// mpConsume2 on hit; not enough → the effect fizzles but the cast already played.
	if skill.MpConsume2 > 0 {
		if int(char.CurrentMP) < skill.MpConsume2 {
			gl.sendToPlayer(caster, outclient.BuildSystemMessageNoParams(outclient.SysMsgNotEnoughMp))
		} else {
			char.CurrentMP -= float64(skill.MpConsume2)
		}
	}

	// Launch packet (resolved targets), then effects.
	gl.broadcastToNearby(caster.Position, outclient.BuildMagicSkillLaunched(e.CharID, cast.SkillID, cast.SkillLevel, []int32{cast.TargetID}))

	gl.applySkillEffects(caster, cast.TargetID, skill)

	// Arm and broadcast the cooldown.
	gl.armSkillReuse(e.CharID, cast.SkillID, cast.SkillLevel, skill.ReuseDelay)
}

// applySkillEffects applies a skill's instant effects to the target. Restore
// effects (Heal/Mp/Cp) route through the existing vitals-restore path; offensive
// damage effects deal magic/physical damage to an NPC target. Buffs/abnormals with
// duration are a later phase (l2go-c8t).
func (gl *GameLoop) applySkillEffects(caster *registry.PlayerWorldState, targetID int32, skill *models.Skill) {
	// Continuous skills (buffs/toggles/HoT/DoT) apply a lasting effect instead of an
	// instant one (l2go-c8t).
	if isBuffSkill(skill) {
		gl.applyBuff(targetID, skill)
		return
	}

	var hp, mp, cp, magicPower, physPower, drainPower int
	for _, eff := range skill.Effects {
		if eff.Scope != models.ScopeGeneral && eff.Scope != models.ScopeSelf {
			continue
		}
		switch eff.Name {
		case "Heal", "Hp", "HealPercent":
			hp += effectPower(eff)
		case "ManaHeal", "Mp", "ManaHealPercent":
			mp += effectPower(eff)
		case "Cp", "CpHeal", "CpHealPercent":
			cp += effectPower(eff)
		case "MagicalAttack", "MagicalAttackRange", "MagicalAttackMp", "MagicalAttackByAbnormal":
			magicPower += effectPower(eff)
		case "PhysicalAttack", "PhysicalAttackHpLink", "PhysicalAttackMute":
			physPower += effectPower(eff)
		case "HpDrain", "DeathLink":
			drainPower += effectPower(eff)
		}
	}

	// Restore effects target a player (self or friendly). SkillID 0 → no extra cast
	// visual (we already sent MagicSkillUse).
	if hp > 0 || mp > 0 || cp > 0 {
		if _, isPlayer := gl.world.GetPlayer(targetID); isPlayer {
			gl.handleRestoreStats(CmdRestoreStats{CharID: targetID, HP: int32(hp), MP: int32(mp), CP: int32(cp)})
		}
	}

	if magicPower <= 0 && physPower <= 0 && drainPower <= 0 {
		return
	}
	casterStats := gl.computePlayerStats(caster)

	// PvP: target is another player. The cast-time gate (handleCastRequest) already
	// validated checkPvpSkill and flagged the attacker; here we deal the damage and
	// flag the victim (retaliation is then free).
	if tgt, isPlayer := gl.world.GetPlayer(targetID); isPlayer && targetID != caster.CharID {
		if tgt.Character == nil || tgt.Character.CurrentHP <= 0 {
			return
		}
		defStats := gl.computePlayerStats(tgt)
		total := 0
		if magicPower > 0 {
			total += calcMagicDamage(float64(casterStats.MAtk), float64(defStats.MDef), magicPower)
		}
		if physPower > 0 {
			total += calcPhysSkillDamage(casterStats.PAtk, defStats.PDef, physPower)
		}
		drainDmg := 0
		if drainPower > 0 {
			drainDmg = calcMagicDamage(float64(casterStats.MAtk), float64(defStats.MDef), drainPower)
			total += drainDmg
		}
		gl.dealDamageToPlayer(tgt, caster.CharID, total)
		gl.setPvPFlag(tgt)
		gl.sendToPlayer(caster, outclient.NewSystemMessage(outclient.SysMsgC1DoneS3DamageToC2).
			AddPlayerName(caster.Character.Name).
			AddPlayerName(tgt.Character.Name).
			AddInt(int32(total)).
			Build())
		if drainDmg > 0 && caster.Character.CurrentHP > 0 {
			gl.handleRestoreStats(CmdRestoreStats{CharID: caster.CharID, HP: int32(drainDmg / 2)})
		}
		return
	}

	// PvE: NPC target.
	npc, isNPC := gl.world.GetNPC(targetID)
	if !isNPC || npc.IsDead || npc.Template == nil {
		return
	}
	if magicPower > 0 {
		gl.dealSkillDamageToNPC(caster, npc, calcMagicDamage(float64(casterStats.MAtk), npc.Template.MDef, magicPower))
	}
	if physPower > 0 {
		gl.dealSkillDamageToNPC(caster, npc, calcPhysSkillDamage(casterStats.PAtk, int(npc.Template.PDef), physPower))
	}
	if drainPower > 0 {
		dmg := calcMagicDamage(float64(casterStats.MAtk), npc.Template.MDef, drainPower)
		gl.dealSkillDamageToNPC(caster, npc, dmg)
		// Drain: heal the caster for half the damage dealt (L2J absorbs a share).
		if caster.Character.CurrentHP > 0 {
			gl.handleRestoreStats(CmdRestoreStats{CharID: caster.CharID, HP: int32(dmg / 2)})
		}
	}
}

// dealSkillDamageToNPC applies skill damage to an NPC and reports it to the caster.
func (gl *GameLoop) dealSkillDamageToNPC(caster *registry.PlayerWorldState, npc *models.NpcInstance, damage int) {
	gl.dealDamageToNPC(npc, caster.CharID, damage)
	gl.sendToPlayer(caster, outclient.NewSystemMessage(outclient.SysMsgC1DoneS3DamageToC2).
		AddPlayerName(caster.Character.Name).
		AddNpcName(npc.TemplateID).
		AddInt(int32(damage)).
		Build())
}

// calcPhysSkillDamage is a physical-skill damage approximation: (76 * (pAtk + power))
// / pDef, mirroring the auto-attack formula plus the skill's power. Floored, min 1.
func calcPhysSkillDamage(pAtk, pDef, power int) int {
	if pDef < 1 {
		pDef = 1
	}
	damage := (76 * (pAtk + power)) / pDef
	if damage < 1 {
		damage = 1
	}
	return damage
}

// calcMagicDamage is L2J Formulas.calcMagicDam (High Five) without shield/crit/
// resist refinements: damage = (91 * sqrt(mAtk) / mDef) * power, floored, min 1.
func calcMagicDamage(mAtk, mDef float64, power int) int {
	if mDef < 1 {
		mDef = 1
	}
	damage := int((91.0 * math.Sqrt(mAtk) / mDef) * float64(power))
	if damage < 1 {
		damage = 1
	}
	return damage
}

// effectPower reads the (already level-resolved) "power"/"amount" param of an effect.
func effectPower(eff models.SkillEffect) int {
	for _, key := range []string{"power", "amount"} {
		if v, ok := eff.Params[key]; ok {
			return parseIntFloor(v)
		}
	}
	return 0
}

// isOffensiveSkill reports whether the skill deals damage / debuffs (targets enemies).
func isOffensiveSkill(skill *models.Skill) bool {
	if skill.IsDebuff {
		return true
	}
	for _, eff := range skill.Effects {
		switch eff.Name {
		case "MagicalAttack", "MagicalAttackRange", "MagicalAttackMp", "MagicalAttackByAbnormal",
			"PhysicalAttack", "PhysicalAttackHpLink", "PhysicalAttackMute", "DeathLink", "Blow":
			return true
		}
	}
	return false
}

// --- skill reuse store (separate from item reuse) ---

func (gl *GameLoop) isSkillOnReuse(charID, skillID int32) bool {
	byID := gl.skillReuse[charID]
	if byID == nil {
		return false
	}
	readyAt, ok := byID[skillID]
	return ok && time.Now().Before(readyAt)
}

func (gl *GameLoop) armSkillReuse(charID, skillID, skillLevel int32, reuseMillis int) {
	if reuseMillis <= 0 {
		return
	}
	byID := gl.skillReuse[charID]
	if byID == nil {
		byID = make(map[int32]time.Time)
		gl.skillReuse[charID] = byID
	}
	byID[skillID] = time.Now().Add(time.Duration(reuseMillis) * time.Millisecond)

	if conn := gl.connections.GetConnection(gl.accountOf(charID)); conn != nil {
		_ = conn.Send(outclient.BuildSkillCoolTime([]outclient.SkillReuseEntry{{
			SkillID:      skillID,
			SkillLevel:   skillLevel,
			ReuseMillis:  int32(reuseMillis),
			RemainMillis: int32(reuseMillis),
		}}))
	}
}

func (gl *GameLoop) accountOf(charID int32) string {
	if p, ok := gl.world.GetPlayer(charID); ok {
		return p.AccountName
	}
	return ""
}

func parseIntFloor(s string) int {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int(f)
}
