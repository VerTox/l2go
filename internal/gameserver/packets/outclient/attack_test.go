package outclient

import (
	"bytes"
	"testing"
)

// TestAttackFlagConstants locks the hit-flag bits to L2J HF Hit.java:
// HITFLAG_USESS=0x10, HITFLAG_CRIT=0x20, HITFLAG_SHLD=0x40, HITFLAG_MISS=0x80.
func TestAttackFlagConstants(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"USESS", AttackFlagSS, 0x10},
		{"CRIT", AttackFlagCrit, 0x20},
		{"SHIELD", AttackFlagShield, 0x40},
		{"MISS", AttackFlagMiss, 0x80},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s flag = 0x%02x, want 0x%02x", c.name, c.got, c.want)
		}
	}
}

// TestBuildAttack_ByteExact verifies the single-hit wire layout against L2J HF
// Attack.writeImpl: C opcode, D attackerId, [D targetId, D damage, C flags],
// D,D,D attackerLoc, H (hits-1), D,D,D targetLoc. Flags byte carries the crit bit.
func TestBuildAttack_ByteExact(t *testing.T) {
	got := BuildAttack(1000, 2000, 150, AttackFlagCrit, 10, 20, 30, 40, 50, 60)
	want := []byte{
		0x33,                   // opcode
		0xE8, 0x03, 0x00, 0x00, // attackerObjId = 1000
		0xD0, 0x07, 0x00, 0x00, // targetObjId = 2000
		0x96, 0x00, 0x00, 0x00, // damage = 150
		0x20,                   // flags = CRIT
		0x0A, 0x00, 0x00, 0x00, // atk X
		0x14, 0x00, 0x00, 0x00, // atk Y
		0x1E, 0x00, 0x00, 0x00, // atk Z
		0x00, 0x00, // hit count - 1 = 0
		0x28, 0x00, 0x00, 0x00, // tgt X
		0x32, 0x00, 0x00, 0x00, // tgt Y
		0x3C, 0x00, 0x00, 0x00, // tgt Z
	}
	if !bytes.Equal(got, want) {
		t.Errorf("BuildAttack bytes mismatch\n got: %x\nwant: %x", got, want)
	}
}

// A soulshot hit ORs the weapon grade id into the USESS flag byte
// (L2J: _flags |= HITFLAG_USESS | ssGrade). Grade S = 5 -> 0x10|5 = 0x15.
func TestBuildAttack_SoulshotFlagWithGrade(t *testing.T) {
	const gradeS = 5
	flags := AttackFlagSS | gradeS
	got := BuildAttack(1, 2, 100, int32(flags), 0, 0, 0, 0, 0, 0)
	if got[13] != 0x15 {
		t.Errorf("soulshot flags byte = 0x%02x, want 0x15", got[13])
	}
}
