package registry

import "testing"

const twoClassTree = `<list>
	<skillTree type="classSkillTree" classId="0">
		<skill skillName="Lucky" skillId="194" skillLvl="1" getLevel="1" autoGet="true" />
		<skill skillName="Common Craft" skillId="1322" skillLvl="1" getLevel="1" autoGet="true" />
		<skill skillName="Expertise D" skillId="239" skillLvl="1" getLevel="20" autoGet="true" />
		<skill skillName="Expertise C" skillId="239" skillLvl="2" getLevel="40" autoGet="true" />
		<skill skillName="Power Strike" skillId="3" skillLvl="1" getLevel="5" levelUpSp="50" learnedByNpc="true" />
	</skillTree>
	<skillTree type="classSkillTree" classId="1" parentClassId="0">
		<skill skillName="Warrior Passive" skillId="500" skillLvl="1" getLevel="20" autoGet="true" />
	</skillTree>
</list>`

func loadTree(t *testing.T) *SkillTreeData {
	t.Helper()
	r := NewSkillTreeData()
	if err := r.load([]byte(twoClassTree)); err != nil {
		t.Fatalf("load: %v", err)
	}
	return r
}

func TestAutoGetSkills_LevelGating(t *testing.T) {
	r := loadTree(t)

	// Level 1: only getLevel<=1 autoGet skills; learnedByNpc excluded.
	got := r.AutoGetSkills(0, 1)
	if len(got) != 2 {
		t.Fatalf("lvl1 = %+v, want 2 (Lucky, Common Craft)", got)
	}
	// Sorted by skill id: 194, 1322.
	if got[0].SkillID != 194 || got[1].SkillID != 1322 {
		t.Errorf("lvl1 ids = %d,%d want 194,1322", got[0].SkillID, got[1].SkillID)
	}

	// Power Strike (learnedByNpc, not autoGet) is never auto-granted.
	for _, s := range r.AutoGetSkills(0, 85) {
		if s.SkillID == 3 {
			t.Error("learnedByNpc Power Strike should not be auto-granted")
		}
	}
}

func TestAutoGetSkills_MaxLevelDedup(t *testing.T) {
	r := loadTree(t)

	// Level 20: Expertise D (lvl1, getLevel20) applies, not yet C.
	got := findSkill(r.AutoGetSkills(0, 20), 239)
	if got == nil || got.Level != 1 {
		t.Fatalf("Expertise@20 = %+v, want level 1", got)
	}
	// Level 45: both D(1) and C(2) apply -> deduped to the highest (2).
	got = findSkill(r.AutoGetSkills(0, 45), 239)
	if got == nil || got.Level != 2 {
		t.Fatalf("Expertise@45 = %+v, want level 2", got)
	}
}

func TestAutoGetSkills_ParentInheritance(t *testing.T) {
	r := loadTree(t)

	// A level-20 class-1 character inherits class-0's autoGet skills plus its own.
	got := r.AutoGetSkills(1, 20)
	ids := map[int32]bool{}
	for _, s := range got {
		ids[s.SkillID] = true
	}
	// Own: 500 (Warrior Passive @20). Inherited: 194, 1322 (@1), 239 D (@20).
	for _, want := range []int32{194, 1322, 239, 500} {
		if !ids[want] {
			t.Errorf("class 1 @20 missing inherited/own skill %d (got %+v)", want, got)
		}
	}
}

func TestAutoGetSkills_UnknownClass(t *testing.T) {
	r := loadTree(t)
	if got := r.AutoGetSkills(999, 80); len(got) != 0 {
		t.Errorf("unknown class = %+v, want empty", got)
	}
}

func findSkill(skills []AutoGetSkill, id int32) *AutoGetSkill {
	for i := range skills {
		if skills[i].SkillID == id {
			return &skills[i]
		}
	}
	return nil
}
