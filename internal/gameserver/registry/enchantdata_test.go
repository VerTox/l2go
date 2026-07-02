package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnchantDataRegistry_LoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "enchantItemData.xml")
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<list>
	<enchant id="955" targetGrade="D" />
	<enchant id="956" targetGrade="D" scrollGroupId="0" />
	<enchant id="959" targetGrade="S" maxEnchant="10" bonusRate="5.5" />
	<enchant id="0" targetGrade="A" />
</list>`
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewEnchantDataRegistry()
	if err := r.LoadFromFile(filepath.Join(dir, "missing.xml"), path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	// id=0 entries are skipped.
	if r.Count() != 3 {
		t.Fatalf("Count = %d, want 3", r.Count())
	}

	d, ok := r.GetEnchantScroll(955)
	if !ok || d.TargetGrade != GradeD || d.MaxEnchant != 65535 {
		t.Fatalf("scroll 955 = %+v ok=%v, want grade D, maxEnchant default 65535", d, ok)
	}

	d, ok = r.GetEnchantScroll(959)
	if !ok || d.TargetGrade != GradeS || d.MaxEnchant != 10 || d.BonusRate != 5.5 {
		t.Fatalf("scroll 959 = %+v, want grade S, maxEnchant 10, bonus 5.5", d)
	}

	if _, ok := r.GetEnchantScroll(999); ok {
		t.Fatalf("unknown scroll should not be found")
	}
}
