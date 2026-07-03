package client

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// fakeSkillSource returns a passive template for the ids in its set, active otherwise.
type fakeSkillSource struct {
	passive map[int]bool
	known   map[int]bool
}

func (f fakeSkillSource) GetSkill(skillID, level int) *models.Skill {
	if f.known != nil && !f.known[skillID] {
		return nil // unknown skill id
	}
	op := models.OpA1
	if f.passive[skillID] {
		op = models.OpP
	}
	return &models.Skill{ID: skillID, Level: level, OperateType: op}
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
