package outclient

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// checkGolden фиксирует байтовый вывод пакета (характеризующий тест).
// Запусти с UPDATE_GOLDEN=1 чтобы (пере)сгенерировать эталоны.
func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run UPDATE_GOLDEN=1 to create)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s bytes mismatch\n got: %x\nwant: %x", name, got, want)
	}
}

func TestCharCreateOk(t *testing.T) {
	checkGolden(t, "charcreateok_true", NewCharCreateOk(true))
	checkGolden(t, "charcreateok_false", NewCharCreateOk(false))
}

func TestCharCreateFail(t *testing.T) {
	checkGolden(t, "charcreatefail_nameexists", NewCharCreateFail(CharCreateFailReasonNameExists))
	checkGolden(t, "charcreatefail_toomany", NewCharCreateFail(CharCreateFailReasonTooManyChars))
}

func TestAcquireSkill(t *testing.T) {
	checkGolden(t, "acquireskilllist", BuildAcquireSkillList([]AcquireSkillEntry{{ID: 3, Level: 1, SP: 50, HasReq: false}}))
	checkGolden(t, "acquireskillinfo", BuildAcquireSkillInfo(3, 1, 50))
	checkGolden(t, "acquireskilldone", BuildAcquireSkillDone())
}

func TestCharList(t *testing.T) {
	checkGolden(t, "charlist", NewCharList())
}

func TestCryptInit(t *testing.T) {
	checkGolden(t, "cryptinit", NewCryptInitPacket(DefaultXORKey()))
}

func TestNewCharacterSuccess(t *testing.T) {
	checkGolden(t, "newcharsuccess_empty", NewCharacterSuccess())
	checkGolden(t, "newcharsuccess_templates", NewCharacterSuccessWithTemplates([]CharTemplate{
		{Race: 1, ClassID: 2, STR: 3, DEX: 4, CON: 5, INT: 6, WIT: 7, MEN: 8,
			StartingX: 100, StartingY: 200, StartingZ: -300, MaxHP: 150, MaxMP: 75},
	}))
}

func TestKeyPacket(t *testing.T) {
	key16 := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	checkGolden(t, "keypacket_ok", NewKeyPacket(key16, true, 1, 0xCAFEBABE))
	checkGolden(t, "keypacket_fail", NewKeyPacket(key16, false, 7, 0))
}

func TestSkillList(t *testing.T) {
	checkGolden(t, "skilllist_empty", NewEmptySkillList())
	checkGolden(t, "skilllist_basic", NewBasicSkillList())
	checkGolden(t, "skilllist_flags", NewSkillList([]SkillInfo{
		{SkillID: 100, SkillLevel: 5, IsPassive: true, IsDisabled: false, IsEnchanted: true},
		{SkillID: 200, SkillLevel: 1, IsPassive: false, IsDisabled: true, IsEnchanted: false},
	}))
}

func TestInventoryUpdate(t *testing.T) {
	checkGolden(t, "inventoryupdate_empty", BuildInventoryUpdate(InventoryUpdate{}))
	checkGolden(t, "inventoryupdate_items", BuildInventoryUpdate(InventoryUpdate{
		Items: []InventoryItem{
			{
				UpdateType: UpdateTypeModify, ObjectID: 268480001, ItemID: 57, LocationSlot: -1,
				Count: 1234567890123, ItemType: 4, CustomType1: 11, Equipped: true, BodyPart: 0x4000,
				EnchantLevel: 6, CustomType2: 22, AugmentationID: 33, Mana: -1, TimeRemaining: -9999,
				AttackElementType: 1, AttackElementPower: 50, DefenseElementFire: 2, DefenseElementWater: 3,
				DefenseElementWind: 4, DefenseElementEarth: 5, DefenseElementHoly: 6, DefenseElementDark: 7,
				EnchantOption1: 8, EnchantOption2: 9, EnchantOption3: 10,
			},
		},
	}))
}
