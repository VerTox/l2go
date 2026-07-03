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

	// MP cost (initial). L2J splits mpConsume1 (on begin) / mpConsume2 (on hit).
	if int(char.CurrentMP) < skill.MpConsume1 {
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

	hitTime := castTime(skill)

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

// castTime returns the cast duration, clamped to a small minimum so instant skills
// still round-trip a launch. (Casting-speed modifiers are a later refinement.)
func castTime(skill *models.Skill) time.Duration {
	d := time.Duration(skill.HitTime) * time.Millisecond
	if d < minCastTime {
		return minCastTime
	}
	return d
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

	var hp, mp, cp, dmgPower int
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
			dmgPower += effectPower(eff)
		}
	}

	// Restore effects target a player (self or friendly). SkillID 0 → no extra cast
	// visual (we already sent MagicSkillUse).
	if hp > 0 || mp > 0 || cp > 0 {
		if _, isPlayer := gl.world.GetPlayer(targetID); isPlayer {
			gl.handleRestoreStats(CmdRestoreStats{CharID: targetID, HP: int32(hp), MP: int32(mp), CP: int32(cp)})
		}
	}

	// Offensive magic damage on an NPC target.
	if dmgPower > 0 {
		gl.applyMagicDamage(caster, targetID, dmgPower)
	}
}

// applyMagicDamage computes and applies a magic-attack skill's damage to an NPC
// target, then reports it. Formula mirrors L2J Formulas.calcMagicDam (High Five):
//
//	damage = (91 * sqrt(mAtk) / mDef) * power
//
// Spiritshot/crit multipliers and magic-resist are later refinements.
func (gl *GameLoop) applyMagicDamage(caster *registry.PlayerWorldState, targetID int32, power int) {
	npc, ok := gl.world.GetNPC(targetID)
	if !ok || npc.IsDead || npc.Template == nil {
		return
	}
	damage := calcMagicDamage(float64(gl.computePlayerStats(caster).MAtk), npc.Template.MDef, power)
	gl.dealDamageToNPC(npc, caster.CharID, damage)

	// "C1 done S3 damage to C2" to the caster.
	gl.sendToPlayer(caster, outclient.NewSystemMessage(outclient.SysMsgC1DoneS3DamageToC2).
		AddPlayerName(caster.Character.Name).
		AddNpcName(npc.TemplateID).
		AddInt(int32(damage)).
		Build())
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
