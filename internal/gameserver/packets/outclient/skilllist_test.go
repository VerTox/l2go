package outclient

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestSkillListByteLayout pins the exact wire format against L2J High Five
// SkillList.writeImpl: C(0x5F) D(count) then per skill D(passive) D(level) D(id)
// C(disabled) C(enchanted).
func TestSkillListByteLayout(t *testing.T) {
	got := NewSkillList([]SkillInfo{
		{SkillID: 1011, SkillLevel: 3, IsPassive: false, IsDisabled: false, IsEnchanted: false},
		{SkillID: 1177, SkillLevel: 101, IsPassive: true, IsDisabled: true, IsEnchanted: true},
	})

	var want bytes.Buffer
	want.WriteByte(0x5f)
	writeD := func(v int32) {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32(v))
		want.Write(b[:])
	}
	writeD(2) // count
	// skill 1: active, level 3, id 1011, not disabled, not enchanted
	writeD(0)    // passive
	writeD(3)    // level
	writeD(1011) // id
	want.WriteByte(0)
	want.WriteByte(0)
	// skill 2: passive, level 101, id 1177, disabled, enchanted
	writeD(1)    // passive
	writeD(101)  // level
	writeD(1177) // id
	want.WriteByte(1)
	want.WriteByte(1)

	if !bytes.Equal(got, want.Bytes()) {
		t.Errorf("SkillList bytes mismatch\n got: %x\nwant: %x", got, want.Bytes())
	}

	// Size check: opcode + count + 2 skills * (3*4 + 2*1) = 1 + 4 + 2*14 = 33.
	if len(got) != 33 {
		t.Errorf("len = %d, want 33", len(got))
	}
}

func TestEmptySkillListBytes(t *testing.T) {
	got := NewEmptySkillList()
	// C(0x5F) + D(0)
	want := []byte{0x5f, 0, 0, 0, 0}
	if !bytes.Equal(got, want) {
		t.Errorf("empty SkillList = %x, want %x", got, want)
	}
}
