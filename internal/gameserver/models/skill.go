package models

// SkillOperateType classifies how a skill is used, mirroring L2J's
// SkillOperateType enum. Only the subset the datapack actually uses is defined;
// unknown values are kept verbatim (helper predicates return false for them).
type SkillOperateType string

const (
	OpA1  SkillOperateType = "A1"  // active, single-shot instant
	OpA2  SkillOperateType = "A2"  // active, continuous (applies an effect over time)
	OpA3  SkillOperateType = "A3"  // active, self-continuous
	OpA4  SkillOperateType = "A4"  // active, continuous (aura-like)
	OpCA1 SkillOperateType = "CA1" // continuous active, channeling
	OpCA5 SkillOperateType = "CA5" // continuous active, channeling
	OpDA1 SkillOperateType = "DA1" // direct active, fly
	OpDA2 SkillOperateType = "DA2" // direct active, fly + continuous
	OpP   SkillOperateType = "P"   // passive
	OpT   SkillOperateType = "T"   // toggle
)

// IsActive reports whether the skill is actively cast (as opposed to passive/toggle).
func (t SkillOperateType) IsActive() bool {
	switch t {
	case OpA1, OpA2, OpA3, OpA4, OpCA1, OpCA5, OpDA1, OpDA2:
		return true
	default:
		return false
	}
}

// IsContinuous reports whether the skill applies a lasting effect on its target.
func (t SkillOperateType) IsContinuous() bool {
	switch t {
	case OpA2, OpA4, OpDA2:
		return true
	default:
		return false
	}
}

// IsSelfContinuous reports whether the skill applies a lasting effect on the caster.
func (t SkillOperateType) IsSelfContinuous() bool { return t == OpA3 }

// IsPassive reports whether the skill is passive (always-on, never cast).
func (t SkillOperateType) IsPassive() bool { return t == OpP }

// IsToggle reports whether the skill is a toggle.
func (t SkillOperateType) IsToggle() bool { return t == OpT }

// IsChanneling reports whether the skill channels while active.
func (t SkillOperateType) IsChanneling() bool { return t == OpCA1 || t == OpCA5 }

// IsFlyType reports whether the skill moves the caster (jump/charge).
func (t SkillOperateType) IsFlyType() bool { return t == OpDA1 || t == OpDA2 }

// TargetType selects the object a skill applies to. Kept as a string-typed enum:
// well-known values are named, but any datapack value is preserved as-is.
type TargetType string

const (
	TargetSelf       TargetType = "SELF"
	TargetTarget     TargetType = "TARGET"
	TargetOne        TargetType = "ONE"
	TargetParty      TargetType = "PARTY"
	TargetClan       TargetType = "CLAN"
	TargetAura       TargetType = "AURA"
	TargetGround     TargetType = "GROUND"
	TargetCorpse     TargetType = "CORPSE"
	TargetSummon     TargetType = "SUMMON"
	TargetEnemy      TargetType = "ENEMY"
	TargetFrontAura  TargetType = "FRONT_AURA"
	TargetBehindAura TargetType = "BEHIND_AURA"
)

// AbnormalType identifies the buff/debuff slot a skill's effect occupies; two
// abnormals of the same type do not stack. Preserved verbatim from the datapack.
type AbnormalType string

const AbnormalNone AbnormalType = "NONE"

// EffectScope tags when/where a skill's effects apply, mirroring L2J's EffectScope.
type EffectScope string

const (
	ScopeGeneral    EffectScope = "GENERAL"
	ScopePassive    EffectScope = "PASSIVE"
	ScopeSelf       EffectScope = "SELF"
	ScopeStart      EffectScope = "START"
	ScopeEnd        EffectScope = "END"
	ScopePve        EffectScope = "PVE"
	ScopePvp        EffectScope = "PVP"
	ScopeChanneling EffectScope = "CHANNELING"
)

// SkillFunc is a stat modifier attached to a skill effect (L2J <add>/<mul>/<sub>/
// <div>/<set> under an <effect>). The value is resolved for the concrete level.
type SkillFunc struct {
	Op    string // "add" | "mul" | "sub" | "div" | "set"
	Stat  string
	Val   float64
	Order int // application order (-1 if unspecified)
}

// SkillEffect is a single declared effect of a skill, resolved for one level.
// P0 only COLLECTS effects (name + params + stat funcs); execution is a later phase.
type SkillEffect struct {
	Name   string            // L2J effect handler name (e.g. "Heal", "Buff", "ConsumeMp")
	Scope  EffectScope       // when the effect applies
	Params map[string]string // <param> values, table refs resolved for this level
	Funcs  []SkillFunc       // <add>/<mul>/... stat modifiers
}

// Skill is a single (id, level) skill template parsed from the L2J datapack. It is
// a subset of L2J's Skill: enough to drive casting, stat mods and effect wiring in
// later phases, without conditions/enchant routes/affect scopes.
type Skill struct {
	ID           int
	Level        int
	DisplayID    int
	DisplayLevel int
	Name         string

	OperateType SkillOperateType
	Magic       int        // isMagic: 0 physical, 1 magic, 2 static, 3 dance/song
	TargetType  TargetType

	CastRange   int // -1 if unset
	EffectRange int // -1 if unset

	HitTime   int // cast time, ms
	CoolTime  int // post-cast lock, ms
	ReuseDelay int // ms

	MpConsume1 int // initial MP cost
	MpConsume2 int // MP cost on cast completion
	HpConsume  int

	ItemConsumeID    int
	ItemConsumeCount int

	EffectPoint int // debuff land power (negative for debuffs)

	AbnormalType AbnormalType
	AbnormalLvl  int
	AbnormalTime int // seconds
	IsDebuff     bool

	MagicLevel   int
	ActivateRate int // -1 if unset

	Effects []SkillEffect
}

// IsMagic reports whether the skill deals/uses magic (isMagic == 1).
func (s *Skill) IsMagic() bool { return s.Magic == 1 }

// IsPassive reports whether the skill is passive.
func (s *Skill) IsPassive() bool { return s.OperateType.IsPassive() }

// IsToggle reports whether the skill is a toggle.
func (s *Skill) IsToggle() bool { return s.OperateType.IsToggle() }
