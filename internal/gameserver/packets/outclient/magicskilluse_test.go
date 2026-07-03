package outclient

import (
	"encoding/binary"
	"testing"
)

// TestMagicSkillUse_TargetLocationDistinct guards the ranged-cast fix: the final
// target location must be the target's real position, not the caster's, so the
// client does not snap a mob to the caster (L2J writeLoc(_target)).
func TestMagicSkillUse_TargetLocation(t *testing.T) {
	// caster at (100,200,-300), target at (900,800,-300).
	pkt := BuildMagicSkillUse(1, 2, 1177, 1, 4000, 2000, 100, 200, -300, 900, 800, -300)

	// Layout: C + D*6 (obj,obj,skill,lvl,hit,reuse)=1+24=25; caster loc D*3 at 25..37;
	// H+H at 37..41; target loc D*3 at 41..53.
	readD := func(off int) int32 { return int32(binary.LittleEndian.Uint32(pkt[off:])) }
	if got := readD(25); got != 100 { // caster x
		t.Errorf("caster x = %d, want 100", got)
	}
	if got := readD(41); got != 900 { // target x
		t.Errorf("target x = %d, want 900 (target, not caster)", got)
	}
	if got := readD(45); got != 800 { // target y
		t.Errorf("target y = %d, want 800", got)
	}
	if len(pkt) != 53 {
		t.Errorf("len = %d, want 53", len(pkt))
	}
}
