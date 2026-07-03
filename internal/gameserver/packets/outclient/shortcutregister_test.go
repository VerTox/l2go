package outclient

import (
	"bytes"
	"testing"
)

// TestBuildShortCutRegister_Item verifies the 0x44 ITEM layout byte-for-byte
// against L2J HF ShortCutRegister.writeImpl:
// C 0x44, D type, D slot+page*12, D id, D characterType, D sharedReuseGroup, D 0, D 0, D 0(augment).
func TestBuildShortCutRegister_Item(t *testing.T) {
	sc := ShortCut{
		Type:             ShortCutTypeItem,
		Slot:             3,
		Page:             1,
		ID:               66,
		CharacterType:    1,
		SharedReuseGroup: -1,
	}
	got := BuildShortCutRegister(sc)
	want := []byte{
		0x44,                   // opcode
		0x01, 0x00, 0x00, 0x00, // type = ITEM
		0x0F, 0x00, 0x00, 0x00, // slot + page*12 = 3 + 12
		0x42, 0x00, 0x00, 0x00, // id = 66
		0x01, 0x00, 0x00, 0x00, // characterType = 1
		0xFF, 0xFF, 0xFF, 0xFF, // sharedReuseGroup = -1
		0x00, 0x00, 0x00, 0x00, // unknown
		0x00, 0x00, 0x00, 0x00, // unknown
		0x00, 0x00, 0x00, 0x00, // item augment id
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ShortCutRegister ITEM mismatch\n got: %x\nwant: %x", got, want)
	}
}

// TestBuildShortCutRegister_Skill: C 0x44, D type, D slot, D id, D level, C 0x00, D characterType.
func TestBuildShortCutRegister_Skill(t *testing.T) {
	sc := ShortCut{
		Type:          ShortCutTypeSkill,
		Slot:          0,
		Page:          0,
		ID:            1177,
		Level:         3,
		CharacterType: 1,
	}
	got := BuildShortCutRegister(sc)
	want := []byte{
		0x44,                   // opcode
		0x02, 0x00, 0x00, 0x00, // type = SKILL
		0x00, 0x00, 0x00, 0x00, // slot
		0x99, 0x04, 0x00, 0x00, // id = 1177
		0x03, 0x00, 0x00, 0x00, // level = 3
		0x00,                   // C5
		0x01, 0x00, 0x00, 0x00, // characterType = 1
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ShortCutRegister SKILL mismatch\n got: %x\nwant: %x", got, want)
	}
}

// TestBuildShortCutRegister_Action: C 0x44, D type, D slot, D id, D characterType.
func TestBuildShortCutRegister_Action(t *testing.T) {
	sc := ShortCut{
		Type:          ShortCutTypeAction,
		Slot:          5,
		Page:          0,
		ID:            7,
		CharacterType: 1,
	}
	got := BuildShortCutRegister(sc)
	want := []byte{
		0x44,                   // opcode
		0x03, 0x00, 0x00, 0x00, // type = ACTION
		0x05, 0x00, 0x00, 0x00, // slot
		0x07, 0x00, 0x00, 0x00, // id = 7
		0x01, 0x00, 0x00, 0x00, // characterType = 1
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ShortCutRegister ACTION mismatch\n got: %x\nwant: %x", got, want)
	}
}
