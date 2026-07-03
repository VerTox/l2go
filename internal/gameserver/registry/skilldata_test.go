package registry

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

func TestSkillHashCode(t *testing.T) {
	// L2J getSkillHashCode: id*1021 + level.
	if got := SkillHashCode(1011, 3); got != 1011*1021+3 {
		t.Errorf("SkillHashCode(1011,3) = %d, want %d", got, 1011*1021+3)
	}
	// Distinct (id, level) pairs must not collide (level < 1021 always).
	if SkillHashCode(1011, 18) == SkillHashCode(1012, 0) {
		t.Error("hash collision between (1011,18) and (1012,0)")
	}
}

// heal1011 mirrors the datapack's tabled Heal skill (18 levels).
const heal1011 = `<list>
	<skill id="1011" levels="18" name="Heal">
		<table name="#effectPoint"> 50 58 67 83 95 107 121 135 151 176 185 195 224 234 245 278 289 301 </table>
		<table name="#healPower"> 50 58 67 83 95 107 121 135 151 176 185 195 224 234 245 278 289 301 </table>
		<table name="#magicLvl"> 3 5 7 10 12 14 16 18 20 23 24 25 28 29 30 33 34 35 </table>
		<table name="#mpConsume1"> 2 3 3 4 4 5 5 6 6 7 7 8 9 9 9 10 10 11 </table>
		<table name="#mpConsume2"> 8 10 11 13 15 17 19 21 24 26 28 29 33 35 35 38 40 41 </table>
		<set name="castRange" val="600" />
		<set name="effectPoint" val="#effectPoint" />
		<set name="effectRange" val="1100" />
		<set name="hitTime" val="5000" />
		<set name="isMagic" val="1" />
		<set name="magicLvl" val="#magicLvl" />
		<set name="mpConsume1" val="#mpConsume1" />
		<set name="mpConsume2" val="#mpConsume2" />
		<set name="operateType" val="A1" />
		<set name="reuseDelay" val="3000" />
		<set name="targetType" val="TARGET" />
		<effects>
			<effect name="Heal">
				<param power="#healPower" />
			</effect>
		</effects>
	</skill>
</list>`

func TestParseTabledSkill_Heal1011(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, root, "01000-01099.xml", heal1011)
	reg := NewSkillData([]string{root})

	// Level 1: first table entries.
	s1 := reg.GetSkill(1011, 1)
	if s1 == nil {
		t.Fatal("GetSkill(1011,1) = nil")
	}
	checks := []struct {
		name string
		got  int
		want int
	}{
		{"ID", s1.ID, 1011},
		{"Level", s1.Level, 1},
		{"DisplayID default=id", s1.DisplayID, 1011},
		{"DisplayLevel default=level", s1.DisplayLevel, 1},
		{"CastRange", s1.CastRange, 600},
		{"EffectRange", s1.EffectRange, 1100},
		{"HitTime", s1.HitTime, 5000},
		{"ReuseDelay", s1.ReuseDelay, 3000},
		{"Magic", s1.Magic, 1},
		{"MpConsume1 lvl1", s1.MpConsume1, 2},
		{"MpConsume2 lvl1", s1.MpConsume2, 8},
		{"MagicLevel lvl1", s1.MagicLevel, 3},
		{"EffectPoint lvl1", s1.EffectPoint, 50},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if s1.Name != "Heal" {
		t.Errorf("Name = %q, want Heal", s1.Name)
	}
	if s1.OperateType != models.OpA1 {
		t.Errorf("OperateType = %q, want A1", s1.OperateType)
	}
	if s1.TargetType != models.TargetTarget {
		t.Errorf("TargetType = %q, want TARGET", s1.TargetType)
	}
	// Coolt/abnormal defaults.
	if s1.CoolTime != 0 || s1.AbnormalType != models.AbnormalNone || s1.ActivateRate != -1 {
		t.Errorf("defaults off: CoolTime=%d AbnormalType=%q ActivateRate=%d", s1.CoolTime, s1.AbnormalType, s1.ActivateRate)
	}

	// Effect collected (name + tabled param resolved at level 1), not executed.
	if len(s1.Effects) != 1 {
		t.Fatalf("len(Effects) = %d, want 1", len(s1.Effects))
	}
	e := s1.Effects[0]
	if e.Name != "Heal" {
		t.Errorf("Effect.Name = %q, want Heal", e.Name)
	}
	if e.Scope != models.ScopeGeneral {
		t.Errorf("Effect.Scope = %q, want GENERAL", e.Scope)
	}
	if e.Params["power"] != "50" {
		t.Errorf("Effect power lvl1 = %q, want 50", e.Params["power"])
	}

	// Level 18: last table entries resolve correctly.
	s18 := reg.GetSkill(1011, 18)
	if s18 == nil {
		t.Fatal("GetSkill(1011,18) = nil")
	}
	if s18.MpConsume1 != 11 {
		t.Errorf("MpConsume1 lvl18 = %d, want 11", s18.MpConsume1)
	}
	if s18.MagicLevel != 35 {
		t.Errorf("MagicLevel lvl18 = %d, want 35", s18.MagicLevel)
	}
	if s18.Effects[0].Params["power"] != "301" {
		t.Errorf("Effect power lvl18 = %q, want 301", s18.Effects[0].Params["power"])
	}
}

func TestGetSkill_LevelClampAndUnknown(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, root, "01000-01099.xml", heal1011)
	reg := NewSkillData([]string{root})

	if got := reg.MaxLevel(1011); got != 18 {
		t.Fatalf("MaxLevel(1011) = %d, want 18", got)
	}
	// Too-high level clamps down to max (L2J parity).
	clamped := reg.GetSkill(1011, 99)
	if clamped == nil || clamped.Level != 18 {
		t.Fatalf("GetSkill(1011,99) clamp = %+v, want level 18", clamped)
	}
	// Unknown skill id -> nil.
	if reg.GetSkill(9999, 1) != nil {
		t.Error("GetSkill(9999,1) should be nil")
	}
	// Unknown range MaxLevel -> 0.
	if reg.MaxLevel(9999) != 0 {
		t.Error("MaxLevel(9999) should be 0")
	}
}

func TestParseBuffFuncs(t *testing.T) {
	root := t.TempDir()
	// Toggle buff (Soul Cry-like): <mul stat> with tabled value + ConsumeMp params.
	writeSkillFile(t, root, "01000-01099.xml", `<list>
		<skill id="1001" levels="2" name="Soul Cry">
			<table name="#pAtk"> 1.5 2.5 </table>
			<set name="operateType" val="T" />
			<set name="targetType" val="SELF" />
			<effects>
				<effect name="ConsumeMp">
					<param power="-2" />
					<param ticks="5" />
				</effect>
				<effect name="Buff">
					<mul stat="pAtk" val="#pAtk" />
				</effect>
			</effects>
		</skill>
	</list>`)
	reg := NewSkillData([]string{root})

	s2 := reg.GetSkill(1001, 2)
	if s2 == nil {
		t.Fatal("GetSkill(1001,2) = nil")
	}
	if !s2.IsToggle() {
		t.Error("expected toggle")
	}
	if len(s2.Effects) != 2 {
		t.Fatalf("len(Effects) = %d, want 2", len(s2.Effects))
	}
	// ConsumeMp params (literal, not tabled).
	if s2.Effects[0].Name != "ConsumeMp" || s2.Effects[0].Params["power"] != "-2" || s2.Effects[0].Params["ticks"] != "5" {
		t.Errorf("ConsumeMp params = %+v", s2.Effects[0].Params)
	}
	// Buff mul func resolves tabled pAtk at level 2 = 2.5.
	buff := s2.Effects[1]
	if len(buff.Funcs) != 1 {
		t.Fatalf("len(Buff.Funcs) = %d, want 1", len(buff.Funcs))
	}
	if buff.Funcs[0].Op != "mul" || buff.Funcs[0].Stat != "pAtk" || buff.Funcs[0].Val != 2.5 {
		t.Errorf("Buff func = %+v, want mul pAtk 2.5", buff.Funcs[0])
	}
}

// TestPassiveModifiersFromRealShape parses a Weapon Mastery-shaped passive (the
// real datapack skill 141: mul pAtk 1.085 + tabled add pAtk) and runs it through
// the 9ep modifier pipeline, bridging the parser (oe1) and stat mods (9ep).
func TestPassiveModifiersFromRealShape(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, root, "00100-00199.xml", `<list>
		<skill id="141" levels="3" name="Weapon Mastery">
			<table name="#pAtk"> 2 3 4 </table>
			<set name="operateType" val="P" />
			<effects>
				<effect name="Buff">
					<mul stat="pAtk" val="1.085" />
					<add stat="pAtk" val="#pAtk" />
				</effect>
			</effects>
		</skill>
	</list>`)
	reg := NewSkillData([]string{root})

	sk := reg.GetSkill(141, 1)
	if sk == nil || !sk.IsPassive() {
		t.Fatalf("GetSkill(141,1) = %+v, want passive", sk)
	}
	mods := models.PassiveModifiers(sk)
	// level 1: add pAtk = #pAtk[0] = 2, plus mul 1.085.
	cs := models.ApplyStatModifiers(models.ComputedStats{PAtk: 100}, mods)
	if cs.PAtk != 111 { // (100 + 2) * 1.085 = 110.67 -> 111
		t.Errorf("PAtk with Weapon Mastery lvl1 = %d, want 111", cs.PAtk)
	}
	// level 3: add pAtk = 4.
	cs3 := models.ApplyStatModifiers(models.ComputedStats{PAtk: 100}, models.PassiveModifiers(reg.GetSkill(141, 3)))
	if cs3.PAtk != 113 { // (100 + 4) * 1.085 = 112.84 -> 113
		t.Errorf("PAtk with Weapon Mastery lvl3 = %d, want 113", cs3.PAtk)
	}
}

func TestPassiveSkillScopeIsPassive(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, root, "00200-00299.xml", `<list>
		<skill id="200" levels="1" name="Passive Might">
			<set name="operateType" val="P" />
			<effects>
				<effect name="Buff">
					<add stat="pAtk" val="10" />
				</effect>
			</effects>
		</skill>
	</list>`)
	reg := NewSkillData([]string{root})
	s := reg.GetSkill(200, 1)
	if s == nil || !s.IsPassive() {
		t.Fatalf("GetSkill(200,1) passive = %+v", s)
	}
	if s.Effects[0].Scope != models.ScopePassive {
		t.Errorf("passive effect scope = %q, want PASSIVE", s.Effects[0].Scope)
	}
	if s.Effects[0].Funcs[0].Val != 10 {
		t.Errorf("add pAtk val = %v, want 10", s.Effects[0].Funcs[0].Val)
	}
}
