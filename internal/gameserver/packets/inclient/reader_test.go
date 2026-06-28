package inclient

import "testing"

// utf16le кодирует строку в UTF-16LE с нулевым терминатором (как протокол L2).
func utf16le(s string) []byte {
	out := make([]byte, 0, len(s)*2+2)
	for _, r := range s {
		out = append(out, byte(r), byte(r>>8))
	}
	out = append(out, 0x00, 0x00)
	return out
}

func TestProtocolVersion(t *testing.T) {
	// 0x01020304 little-endian
	p := NewProtocolVersion([]byte{0x04, 0x03, 0x02, 0x01})
	if p.Version != 0x01020304 {
		t.Errorf("Version = %#x, want %#x", p.Version, 0x01020304)
	}
}

func TestUseItem(t *testing.T) {
	// ObjectID = 0x0A0B0C0D, CtrlPressed = true (non-zero)
	p := NewUseItem([]byte{0x0D, 0x0C, 0x0B, 0x0A, 0x01, 0x00, 0x00, 0x00})
	if p.ObjectID != 0x0A0B0C0D {
		t.Errorf("ObjectID = %#x, want %#x", p.ObjectID, 0x0A0B0C0D)
	}
	if !p.CtrlPressed {
		t.Errorf("CtrlPressed = false, want true")
	}

	p2 := NewUseItem([]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if p2.CtrlPressed {
		t.Errorf("CtrlPressed = true, want false")
	}
}

func TestRequestUnEquipItem(t *testing.T) {
	p := NewRequestUnEquipItem([]byte{0x00, 0x40, 0x00, 0x00})
	if p.SlotBitmask != 0x4000 {
		t.Errorf("SlotBitmask = %#x, want %#x", p.SlotBitmask, 0x4000)
	}
}
