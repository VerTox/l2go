package outclient

import (
	"bytes"
	"testing"
)

// TestBuildShortCutInit_Item locks the 0x45 ITEM layout against L2J HF
// ShortCutInit.writeImpl. The trailing pair is H 0, H 0 (4 bytes) — NOT D 0, D 0.
// Regression guard: the list was only ever sent empty before persistence landed.
func TestBuildShortCutInit_Item(t *testing.T) {
	got := BuildShortCutInit([]ShortCut{{
		Type:             ShortCutTypeItem,
		Slot:             0,
		Page:             0,
		ID:               100,
		SharedReuseGroup: -1,
	}})
	want := []byte{
		0x45,                   // opcode
		0x01, 0x00, 0x00, 0x00, // count = 1
		0x01, 0x00, 0x00, 0x00, // type = ITEM
		0x00, 0x00, 0x00, 0x00, // slot + page*12 = 0
		0x64, 0x00, 0x00, 0x00, // id = 100
		0x01, 0x00, 0x00, 0x00, // 0x01
		0xFF, 0xFF, 0xFF, 0xFF, // sharedReuseGroup = -1
		0x00, 0x00, 0x00, 0x00, // unknown
		0x00, 0x00, 0x00, 0x00, // unknown
		0x00, 0x00, // H 0
		0x00, 0x00, // H 0
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ShortCutInit ITEM mismatch\n got: %x\nwant: %x", got, want)
	}
}

// TestBuildShortCutInit_Empty: header only (count 0).
func TestBuildShortCutInit_Empty(t *testing.T) {
	got := BuildShortCutInit(nil)
	want := []byte{0x45, 0x00, 0x00, 0x00, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("ShortCutInit empty mismatch\n got: %x\nwant: %x", got, want)
	}
}
