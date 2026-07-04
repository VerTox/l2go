package registry

import "testing"

func TestGetLearnableSkills(t *testing.T) {
	xmlData := []byte(`<list>
<skillTree type="classSkillTree" classId="0">
  <skill skillId="3" skillLvl="1" getLevel="5" levelUpSp="50" learnedByNpc="true"/>
  <skill skillId="3" skillLvl="2" getLevel="10" levelUpSp="80" learnedByNpc="true"/>
  <skill skillId="7" skillLvl="1" getLevel="1" autoGet="true"/>
</skillTree></list>`)
	r := NewSkillTreeData()
	if err := r.load(xmlData); err != nil {
		t.Fatal(err)
	}

	// level 6, nothing known: only skill 3 lvl1 (learnedByNpc, getLevel<=6, level==1). autoGet 7 excluded.
	got := r.GetLearnableSkills(0, 6, map[int32]int32{})
	if len(got) != 1 || got[0].SkillID != 3 || got[0].Level != 1 || got[0].LevelUpSp != 50 {
		t.Fatalf("want [skill3 lvl1 sp50], got %+v", got)
	}
	// level 10, knows skill3 lvl1: only skill3 lvl2 (known==level-1).
	got = r.GetLearnableSkills(0, 10, map[int32]int32{3: 1})
	if len(got) != 1 || got[0].SkillID != 3 || got[0].Level != 2 {
		t.Fatalf("want [skill3 lvl2], got %+v", got)
	}
	// GetSkillLearn lookup
	if sl := r.GetSkillLearn(0, 3, 2); sl == nil || sl.LevelUpSp != 80 {
		t.Fatalf("GetSkillLearn(3,2) bad: %+v", sl)
	}
}
