package registry

import (
	"os"
	"path/filepath"
	"testing"
)

// bodyPartToCode values used by the tests (must match itemtemplates.go).
const (
	tstSlotChest    = 0x0400
	tstSlotFull     = 0x8000
	tstSlotRHand    = 0x0080
	tstSlotLRHand   = 0x4000
	tstSlotRFinger  = 0x0010
)

func writeGroupsXML(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "enchantItemGroups.xml")
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<list>
	<enchantRateGroup name="ARMOR_GROUP">
		<current enchant="0-2" chance="100" />
		<current enchant="3" chance="66.67" />
		<current enchant="4" chance="33.34" />
		<current enchant="20-65535" chance="0" />
	</enchantRateGroup>
	<enchantRateGroup name="FULL_ARMOR_GROUP">
		<current enchant="0-3" chance="100" />
		<current enchant="4" chance="66.67" />
	</enchantRateGroup>
	<enchantRateGroup name="FIGHTER_WEAPON_GROUP">
		<current enchant="0-2" chance="100" />
		<current enchant="3-14" chance="70" />
		<current enchant="15-65535" chance="35" />
	</enchantRateGroup>
	<enchantRateGroup name="MAGE_WEAPON_GROUP">
		<current enchant="0-2" chance="100" />
		<current enchant="3-14" chance="40" />
		<current enchant="15-65535" chance="20" />
	</enchantRateGroup>
	<enchantScrollGroup id="0">
		<enchantRate group="ARMOR_GROUP">
			<item slot="chest" />
			<item slot="rfinger;lfinger" />
		</enchantRate>
		<enchantRate group="FULL_ARMOR_GROUP">
			<item slot="fullarmor" />
		</enchantRate>
		<enchantRate group="FIGHTER_WEAPON_GROUP">
			<item slot="rhand" magicWeapon="false" />
			<item slot="lrhand" magicWeapon="false" />
		</enchantRate>
		<enchantRate group="MAGE_WEAPON_GROUP">
			<item slot="rhand" magicWeapon="true" />
			<item slot="lrhand" magicWeapon="true" />
		</enchantRate>
	</enchantScrollGroup>
</list>`
	if err := os.WriteFile(path, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnchantGroupsRegistry_Load(t *testing.T) {
	path := writeGroupsXML(t)
	r := NewEnchantGroupsRegistry()
	if err := r.LoadFromFile(filepath.Join(filepath.Dir(path), "missing.xml"), path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if r.RateGroupCount() != 4 {
		t.Fatalf("rate groups = %d, want 4", r.RateGroupCount())
	}
	if r.ScrollGroupCount() != 1 {
		t.Fatalf("scroll groups = %d, want 1", r.ScrollGroupCount())
	}
}

func TestEnchantGroupsRegistry_Chance(t *testing.T) {
	path := writeGroupsXML(t)
	r := NewEnchantGroupsRegistry()
	if err := r.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	tests := []struct {
		name       string
		bodyPart   int32
		magic      bool
		itemID     int32
		level      int
		wantChance float64
		wantOK     bool
	}{
		{"chest armor lvl4 -> ARMOR_GROUP", tstSlotChest, false, 0, 4, 33.34, true},
		{"ring (rfinger;lfinger) lvl0 -> ARMOR_GROUP", tstSlotRFinger, false, 0, 0, 100, true},
		{"full armor lvl4 -> FULL_ARMOR_GROUP", tstSlotFull, false, 0, 4, 66.67, true},
		{"fighter 1H weapon lvl5 -> FIGHTER 70", tstSlotRHand, false, 0, 5, 70, true},
		{"fighter 2H weapon lvl16 -> FIGHTER 35", tstSlotLRHand, false, 0, 16, 35, true},
		{"mage weapon lvl5 -> MAGE 40", tstSlotRHand, true, 0, 5, 40, true},
		{"armor beyond range lvl25 -> 0", tstSlotChest, false, 0, 25, 0, true},
		{"unknown slot -> no binding", 0x0001, false, 0, 0, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := r.Chance(0, tc.bodyPart, tc.magic, tc.itemID, tc.level)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.wantChance {
				t.Fatalf("chance = %v, want %v", got, tc.wantChance)
			}
		})
	}

	// Unknown scroll group id -> not found.
	if _, ok := r.Chance(99, tstSlotChest, false, 0, 0); ok {
		t.Fatalf("unknown scroll group should return ok=false")
	}
}
