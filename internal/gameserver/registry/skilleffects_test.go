package registry

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSkillFile is a helper that writes an XML skill list to <dir>/<name>.
func writeSkillFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestSkillEffects_TickHpPower(t *testing.T) {
	root := t.TempDir()
	// Greater Healing Potion skill: TickHp effect with power=50 (HP over time).
	// INTERIM: we restore the flat power value instantly.
	writeSkillFile(t, root, "02000-02099.xml", `<list>
		<skill id="2037" levels="1" name="Greater Healing Potion">
			<effects>
				<effect name="TickHp">
					<param power="50" />
					<param ticks="3" />
					<param mode="DIFF" />
				</effect>
			</effects>
		</skill>
	</list>`)

	reg := NewSkillEffectRegistry([]string{root})
	eff, ok := reg.Lookup(2037, 1)
	if !ok {
		t.Fatal("Lookup(2037,1) not found")
	}
	if eff.Kind != EffectHP {
		t.Errorf("Kind = %v, want EffectHP", eff.Kind)
	}
	if eff.Amount != 50 {
		t.Errorf("Amount = %d, want 50", eff.Amount)
	}
}

func TestSkillEffects_InstantMpAmount(t *testing.T) {
	root := t.TempDir()
	// Mana Potion (custom skill, lives in custom/ subdir) restores 100 MP instantly.
	writeSkillFile(t, filepath.Join(root, "custom"), "10000-10099.xml", `<list>
		<skill id="10001" levels="1" name="Mana Potion">
			<effects>
				<effect name="Mp">
					<param amount="100" />
				</effect>
			</effects>
		</skill>
	</list>`)

	reg := NewSkillEffectRegistry([]string{root})
	eff, ok := reg.Lookup(10001, 1)
	if !ok {
		t.Fatal("Lookup(10001,1) not found in custom/ dir")
	}
	if eff.Kind != EffectMP || eff.Amount != 100 {
		t.Errorf("effect = %+v, want {EffectMP, 100}", eff)
	}
}

func TestSkillEffects_MultiLevelTable(t *testing.T) {
	root := t.TempDir()
	// CP Gauge Potion: amount driven by a #table, level 1 -> 50, level 2 -> 200.
	writeSkillFile(t, root, "02100-02199.xml", `<list>
		<skill id="2166" levels="2" name="CP Gauge Potion">
			<table name="#amount"> 50 200 </table>
			<effects>
				<effect name="Cp">
					<param amount="#amount" />
				</effect>
			</effects>
		</skill>
	</list>`)

	reg := NewSkillEffectRegistry([]string{root})

	eff1, ok := reg.Lookup(2166, 1)
	if !ok || eff1.Kind != EffectCP || eff1.Amount != 50 {
		t.Errorf("Lookup(2166,1) = %+v, ok=%v; want {EffectCP,50}", eff1, ok)
	}
	eff2, ok := reg.Lookup(2166, 2)
	if !ok || eff2.Kind != EffectCP || eff2.Amount != 200 {
		t.Errorf("Lookup(2166,2) = %+v, ok=%v; want {EffectCP,200}", eff2, ok)
	}
}

func TestSkillEffects_FractionalPowerFloored(t *testing.T) {
	root := t.TempDir()
	// Mana Drug: TickMp power=1.5 -> floored to 1 (interim single application).
	writeSkillFile(t, filepath.Join(root, "custom"), "10000-10099.xml", `<list>
		<skill id="10000" levels="1" name="Mana Drug">
			<effects>
				<effect name="TickMp">
					<param power="1.5" />
					<param ticks="3" />
				</effect>
			</effects>
		</skill>
	</list>`)

	reg := NewSkillEffectRegistry([]string{root})
	eff, ok := reg.Lookup(10000, 1)
	if !ok || eff.Kind != EffectMP || eff.Amount != 1 {
		t.Errorf("Lookup(10000,1) = %+v, ok=%v; want {EffectMP,1}", eff, ok)
	}
}

func TestSkillEffects_UnknownSkill(t *testing.T) {
	reg := NewSkillEffectRegistry([]string{t.TempDir()})
	if _, ok := reg.Lookup(9999, 1); ok {
		t.Error("Lookup of missing skill returned ok=true")
	}
}
