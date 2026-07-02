package outclient

import (
	"bytes"
	"testing"
)

func TestBuildExUseSharedGroupItem(t *testing.T) {
	// itemId=0x1234, group=42(0x2A), remaining=10s, total=20s.
	got := BuildExUseSharedGroupItem(0x1234, 42, 10, 20)
	want := []byte{
		0xFE,       // opcode
		0x4A, 0x00, // sub-opcode (H, little-endian)
		0x34, 0x12, 0x00, 0x00, // itemId
		0x2A, 0x00, 0x00, 0x00, // groupId
		0x0A, 0x00, 0x00, 0x00, // remaining seconds
		0x14, 0x00, 0x00, 0x00, // total seconds
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ExUseSharedGroupItem bytes mismatch\n got: %x\nwant: %x", got, want)
	}
}
