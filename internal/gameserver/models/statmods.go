package models

import "math"

// StatName identifies a derived stat a modifier targets. Values match the L2J
// datapack stat names used in <add>/<sub>/<mul>/<div stat="..."> skill nodes.
// Only stats present in ComputedStats are modelled; other names (resistances,
// regen, pvp, ...) are recognised as no-ops until ComputedStats grows.
type StatName string

const (
	StatPAtk     StatName = "pAtk"
	StatMAtk     StatName = "mAtk"
	StatPDef     StatName = "pDef"
	StatMDef     StatName = "mDef"
	StatPAtkSpd  StatName = "pAtkSpd"
	StatMAtkSpd  StatName = "mAtkSpd"
	StatAccuracy StatName = "accCombat"
	StatEvasion  StatName = "rEvas"
	StatCritRate StatName = "critRate"
	StatRunSpd   StatName = "runSpd"
	StatWalkSpd  StatName = "walkSpd"
	StatMaxHP    StatName = "maxHp"
	StatMaxMP    StatName = "maxMp"
	StatMaxCP    StatName = "maxCp"
	StatMaxLoad  StatName = "weightLimit"
)

// StatModifier is one add/sub/mul/div/set operation on a stat, sourced from a
// passive skill (this phase) or, later, a timed buff.
type StatModifier struct {
	Stat StatName
	Op   string // "add" | "sub" | "mul" | "div" | "set"
	Val  float64
}

// ModifiersFromFuncs converts parsed skill stat-funcs into StatModifiers.
func ModifiersFromFuncs(funcs []SkillFunc) []StatModifier {
	mods := make([]StatModifier, 0, len(funcs))
	for _, f := range funcs {
		mods = append(mods, StatModifier{Stat: StatName(f.Stat), Op: f.Op, Val: f.Val})
	}
	return mods
}

// PassiveModifiers extracts the stat modifiers a passive skill grants: the stat
// funcs of its PASSIVE-scope effects. Returns nil for non-passive skills.
func PassiveModifiers(skill *Skill) []StatModifier {
	if skill == nil || !skill.IsPassive() {
		return nil
	}
	var mods []StatModifier
	for _, e := range skill.Effects {
		if e.Scope != ScopePassive {
			continue
		}
		mods = append(mods, ModifiersFromFuncs(e.Funcs)...)
	}
	return mods
}

// ApplyStatModifiers returns cs with the modifier set applied. For each stat the
// L2J-simplified formula is used: (base + Σadd − Σsub) × Πmul ÷ Πdiv, with an
// optional "set" overriding the base. This is the common emulator approximation
// (order-independent within a stat) and is exact for percentage/flat buffs.
// Modifiers targeting stats not in ComputedStats are ignored.
func ApplyStatModifiers(cs ComputedStats, mods []StatModifier) ComputedStats {
	if len(mods) == 0 {
		return cs
	}
	byStat := make(map[StatName][]StatModifier)
	for _, m := range mods {
		byStat[m.Stat] = append(byStat[m.Stat], m)
	}

	apply := func(field *int, stat StatName) {
		ms, ok := byStat[stat]
		if !ok {
			return
		}
		*field = int(math.Round(reduceModifiers(float64(*field), ms)))
	}

	apply(&cs.PAtk, StatPAtk)
	apply(&cs.MAtk, StatMAtk)
	apply(&cs.PDef, StatPDef)
	apply(&cs.MDef, StatMDef)
	apply(&cs.PAtkSpd, StatPAtkSpd)
	apply(&cs.MAtkSpd, StatMAtkSpd)
	apply(&cs.Accuracy, StatAccuracy)
	apply(&cs.Evasion, StatEvasion)
	apply(&cs.CritRate, StatCritRate)
	apply(&cs.RunSpd, StatRunSpd)
	apply(&cs.WalkSpd, StatWalkSpd)
	apply(&cs.MaxHP, StatMaxHP)
	apply(&cs.MaxMP, StatMaxMP)
	apply(&cs.MaxCP, StatMaxCP)
	apply(&cs.MaxLoad, StatMaxLoad)
	return cs
}

// reduceModifiers folds one stat's modifiers over a base value.
func reduceModifiers(base float64, ms []StatModifier) float64 {
	value := base
	hasSet := false
	var setVal, addSum, subSum float64
	mulProd, divProd := 1.0, 1.0
	for _, m := range ms {
		switch m.Op {
		case "add":
			addSum += m.Val
		case "sub":
			subSum += m.Val
		case "mul":
			mulProd *= m.Val
		case "div":
			if m.Val != 0 {
				divProd *= m.Val
			}
		case "set":
			setVal, hasSet = m.Val, true
		}
	}
	if hasSet {
		value = setVal
	}
	return (value + addSum - subSum) * mulProd / divProd
}
