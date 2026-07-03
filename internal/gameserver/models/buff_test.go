package models

import (
	"testing"
	"time"
)

func buff(skillID int32, abType AbnormalType, abLvl int, mods ...StatModifier) *BuffInfo {
	return &BuffInfo{SkillID: skillID, SkillLevel: 1, AbnormalType: abType, AbnormalLvl: abLvl, Mods: mods}
}

func TestCharEffectList_AddAndMods(t *testing.T) {
	var l CharEffectList
	if !l.Add(buff(100, "MIGHT", 1, StatModifier{Stat: StatPAtk, Op: "mul", Val: 1.1})) {
		t.Fatal("first add should succeed")
	}
	if !l.Add(buff(200, "SHIELD", 1, StatModifier{Stat: StatPDef, Op: "add", Val: 50})) {
		t.Fatal("second add should succeed")
	}
	if l.Len() != 2 {
		t.Fatalf("Len = %d, want 2", l.Len())
	}
	if len(l.Mods()) != 2 {
		t.Errorf("Mods = %d, want 2", len(l.Mods()))
	}
}

func TestCharEffectList_SameTypeOverride(t *testing.T) {
	var l CharEffectList
	l.Add(buff(100, "PA_UP", 1))

	// Higher abnormalLvl of the same type replaces the old.
	if !l.Add(buff(101, "PA_UP", 2)) {
		t.Fatal("stronger same-type should replace")
	}
	if l.Len() != 1 || l.Buffs()[0].SkillID != 101 {
		t.Fatalf("after override: %+v, want only skill 101", l.Buffs())
	}

	// Weaker same-type is rejected.
	if l.Add(buff(102, "PA_UP", 1)) {
		t.Error("weaker same-type should be rejected")
	}
	if l.Len() != 1 || l.Buffs()[0].SkillID != 101 {
		t.Errorf("weaker add mutated list: %+v", l.Buffs())
	}
}

func TestCharEffectList_SameSkillRefreshes(t *testing.T) {
	var l CharEffectList
	l.Add(buff(100, "NONE", 1))
	l.Add(buff(100, "NONE", 1)) // recast same skill
	if l.Len() != 1 {
		t.Errorf("same skill recast should refresh, Len = %d want 1", l.Len())
	}
}

func TestCharEffectList_NoneTypeNoStacking(t *testing.T) {
	var l CharEffectList
	// AbnormalType NONE must not cross-override different skills.
	l.Add(buff(100, AbnormalNone, 1))
	l.Add(buff(200, AbnormalNone, 1))
	if l.Len() != 2 {
		t.Errorf("NONE type should not override across skills, Len = %d want 2", l.Len())
	}
}

func TestCharEffectList_RemoveExpired(t *testing.T) {
	var l CharEffectList
	now := time.Unix(1000, 0)

	active := buff(100, "A", 1)
	active.ExpiresAt = now.Add(10 * time.Second)
	expired := buff(200, "B", 1)
	expired.ExpiresAt = now.Add(-time.Second)
	infinite := buff(300, "C", 1) // zero ExpiresAt = toggle, never expires
	l.Add(active)
	l.Add(expired)
	l.Add(infinite)

	got := l.RemoveExpired(now)
	if len(got) != 1 || got[0].SkillID != 200 {
		t.Fatalf("expired = %+v, want [200]", got)
	}
	if l.Len() != 2 || l.HasSkill(200) {
		t.Errorf("after RemoveExpired: %+v", l.Buffs())
	}
}

func TestCharEffectList_RemoveSkillToggle(t *testing.T) {
	var l CharEffectList
	l.Add(buff(100, "TOGGLE", 1))
	if !l.RemoveSkill(100) {
		t.Error("RemoveSkill(100) should return true")
	}
	if l.HasSkill(100) {
		t.Error("skill 100 still present after removal")
	}
	if l.RemoveSkill(999) {
		t.Error("RemoveSkill of absent skill should return false")
	}
}
