package client

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// fakeSkillSource returns a passive template for the ids in its set, active otherwise.
type fakeSkillSource struct {
	passive map[int]bool
	known   map[int]bool
	funcs   map[int][]models.SkillFunc
}

func (f fakeSkillSource) GetSkill(skillID, level int) *models.Skill {
	if f.known != nil && !f.known[skillID] {
		return nil // unknown skill id
	}
	op := models.OpA1
	if f.passive[skillID] {
		op = models.OpP
	}
	sk := &models.Skill{ID: skillID, Level: level, OperateType: op}
	if funcs, ok := f.funcs[skillID]; ok {
		scope := models.ScopeGeneral
		if op == models.OpP {
			scope = models.ScopePassive
		}
		sk.Effects = []models.SkillEffect{{Name: "Buff", Scope: scope, Funcs: funcs}}
	}
	return sk
}

func TestBuildSkillInfos(t *testing.T) {
	src := fakeSkillSource{
		passive: map[int]bool{1001: true},
		known:   map[int]bool{1001: true, 1177: true}, // 9999 unknown
	}
	skills := []models.CharacterSkill{
		{SkillID: 1177, SkillLevel: 1},   // active
		{SkillID: 1001, SkillLevel: 3},   // passive
		{SkillID: 1177, SkillLevel: 101}, // enchanted route
		{SkillID: 9999, SkillLevel: 1},   // unknown template -> active fallback
	}

	got := buildSkillInfos(skills, src)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}

	if got[0].SkillID != 1177 || got[0].SkillLevel != 1 || got[0].IsPassive || got[0].IsEnchanted {
		t.Errorf("skill[0] = %+v, want active Wind Strike lvl1", got[0])
	}
	if !got[1].IsPassive || got[1].SkillLevel != 3 {
		t.Errorf("skill[1] = %+v, want passive lvl3", got[1])
	}
	if !got[2].IsEnchanted || got[2].IsPassive {
		t.Errorf("skill[2] = %+v, want enchanted active", got[2])
	}
	if got[3].IsPassive {
		t.Errorf("skill[3] unknown template should default to active, got %+v", got[3])
	}
	// Disabled is always false here (no cooldown modelling yet).
	for i, si := range got {
		if si.IsDisabled {
			t.Errorf("skill[%d] unexpectedly disabled", i)
		}
	}
}

// nil source (skillData not wired) must not panic and yields all-active skills.
func TestBuildSkillInfos_NilSource(t *testing.T) {
	got := buildSkillInfos([]models.CharacterSkill{{SkillID: 1001, SkillLevel: 1}}, nil)
	if len(got) != 1 || got[0].IsPassive {
		t.Fatalf("nil source = %+v, want single active skill", got)
	}
}

func TestCollectPassiveModifiers(t *testing.T) {
	src := fakeSkillSource{
		passive: map[int]bool{300: true},
		known:   map[int]bool{300: true, 1177: true},
		funcs: map[int][]models.SkillFunc{
			300:  {{Op: "mul", Stat: "pAtk", Val: 1.08}}, // passive -> collected
			1177: {{Op: "mul", Stat: "pAtk", Val: 5}},    // active -> ignored
		},
	}
	skills := []models.CharacterSkill{
		{SkillID: 1177, SkillLevel: 1}, // active, funcs not passive-scoped
		{SkillID: 300, SkillLevel: 1},  // passive
		{SkillID: 9999, SkillLevel: 1}, // unknown -> nil template
	}

	mods := collectPassiveModifiers(skills, src)
	if len(mods) != 1 {
		t.Fatalf("len(mods) = %d, want 1 (only the passive skill), got %+v", len(mods), mods)
	}
	if mods[0].Stat != models.StatPAtk || mods[0].Op != "mul" || mods[0].Val != 1.08 {
		t.Errorf("mod = %+v, want mul pAtk 1.08", mods[0])
	}

	// nil source yields no mods, no panic.
	if got := collectPassiveModifiers(skills, nil); got != nil {
		t.Errorf("nil source = %+v, want nil", got)
	}
}
