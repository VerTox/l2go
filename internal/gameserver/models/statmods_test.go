package models

import "testing"

func baseStats() ComputedStats {
	return ComputedStats{PAtk: 100, MAtk: 50, PDef: 200, RunSpd: 120, CritRate: 40, MaxHP: 1000}
}

func TestApplyStatModifiers_Mul(t *testing.T) {
	cs := ApplyStatModifiers(baseStats(), []StatModifier{
		{Stat: StatPAtk, Op: "mul", Val: 1.15},
	})
	if cs.PAtk != 115 { // 100 * 1.15
		t.Errorf("PAtk = %d, want 115", cs.PAtk)
	}
	// Untouched stats unchanged.
	if cs.PDef != 200 || cs.MAtk != 50 {
		t.Errorf("unrelated stats changed: PDef=%d MAtk=%d", cs.PDef, cs.MAtk)
	}
}

func TestApplyStatModifiers_AddAndStack(t *testing.T) {
	// add stacks additively, mul stacks multiplicatively, applied as
	// (base + add) * mul1 * mul2.
	cs := ApplyStatModifiers(baseStats(), []StatModifier{
		{Stat: StatPAtk, Op: "add", Val: 20},  // 100 -> 120
		{Stat: StatPAtk, Op: "mul", Val: 1.5}, // 120 -> 180
		{Stat: StatPAtk, Op: "mul", Val: 2.0}, // 180 -> 360
	})
	if cs.PAtk != 360 {
		t.Errorf("PAtk = %d, want 360", cs.PAtk)
	}
}

func TestApplyStatModifiers_SubAndDiv(t *testing.T) {
	cs := ApplyStatModifiers(baseStats(), []StatModifier{
		{Stat: StatRunSpd, Op: "sub", Val: 20}, // 120 -> 100
		{Stat: StatRunSpd, Op: "div", Val: 2},  // 100 -> 50
	})
	if cs.RunSpd != 50 {
		t.Errorf("RunSpd = %d, want 50", cs.RunSpd)
	}
	// div by zero is ignored (no panic, no change from div).
	cs2 := ApplyStatModifiers(baseStats(), []StatModifier{{Stat: StatRunSpd, Op: "div", Val: 0}})
	if cs2.RunSpd != 120 {
		t.Errorf("div-by-zero RunSpd = %d, want 120", cs2.RunSpd)
	}
}

func TestApplyStatModifiers_EmptyRestoresBase(t *testing.T) {
	base := baseStats()
	if got := ApplyStatModifiers(base, nil); got != base {
		t.Errorf("empty mods changed stats: %+v != %+v", got, base)
	}
}

func TestApplyStatModifiers_UnknownStatIgnored(t *testing.T) {
	base := baseStats()
	got := ApplyStatModifiers(base, []StatModifier{
		{Stat: "darkRes", Op: "add", Val: 30}, // no ComputedStats field
		{Stat: StatPAtk, Op: "mul", Val: 2},   // still applied
	})
	if got.PAtk != 200 {
		t.Errorf("PAtk = %d, want 200", got.PAtk)
	}
}

func TestPassiveModifiers(t *testing.T) {
	// A passive skill with a PASSIVE-scope Buff effect adding pAtk.
	passive := &Skill{
		OperateType: OpP,
		Effects: []SkillEffect{
			{Name: "Buff", Scope: ScopePassive, Funcs: []SkillFunc{
				{Op: "mul", Stat: "pAtk", Val: 1.08},
				{Op: "add", Stat: "pDef", Val: 15},
			}},
		},
	}
	mods := PassiveModifiers(passive)
	if len(mods) != 2 {
		t.Fatalf("len(mods) = %d, want 2", len(mods))
	}

	// An active skill yields no passive modifiers, even with GENERAL funcs.
	active := &Skill{
		OperateType: OpA1,
		Effects:     []SkillEffect{{Name: "Buff", Scope: ScopeGeneral, Funcs: []SkillFunc{{Op: "mul", Stat: "pAtk", Val: 2}}}},
	}
	if got := PassiveModifiers(active); got != nil {
		t.Errorf("active skill PassiveModifiers = %+v, want nil", got)
	}

	// End to end: applying the passive's mods.
	cs := ApplyStatModifiers(ComputedStats{PAtk: 100, PDef: 100}, mods)
	if cs.PAtk != 108 || cs.PDef != 115 {
		t.Errorf("applied = PAtk %d PDef %d, want 108/115", cs.PAtk, cs.PDef)
	}
}
