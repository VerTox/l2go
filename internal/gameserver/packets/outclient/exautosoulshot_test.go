package outclient

import (
	"bytes"
	"testing"
)

// TestBuildExAutoSoulShot verifies the 0xFE:0x0C layout against L2J HF
// ExAutoSoulShot.writeImpl: C 0xFE, H 0x0C, D itemId, D type (1=on, 0=off).
func TestBuildExAutoSoulShot_On(t *testing.T) {
	got := BuildExAutoSoulShot(1803, 1)
	want := []byte{
		0xFE,       // opcode
		0x0C, 0x00, // sub-opcode (H, little-endian)
		0x0B, 0x07, 0x00, 0x00, // itemId = 1803
		0x01, 0x00, 0x00, 0x00, // type = on
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ExAutoSoulShot(on) mismatch\n got: %x\nwant: %x", got, want)
	}
}

func TestBuildExAutoSoulShot_Off(t *testing.T) {
	got := BuildExAutoSoulShot(1803, 0)
	want := []byte{
		0xFE,
		0x0C, 0x00,
		0x0B, 0x07, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, // type = off
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ExAutoSoulShot(off) mismatch\n got: %x\nwant: %x", got, want)
	}
}
