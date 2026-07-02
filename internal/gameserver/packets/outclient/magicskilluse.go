package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildMagicSkillUse builds a MagicSkillUse packet (opcode 0x48).
//
// It is the animation packet L2J broadcasts for skill casts and shot activation
// (soulshot / spiritshot visual). Byte layout matches L2J High Five
// MagicSkillUse.writeImpl:
//
//	C  0x48
//	D  caster object id
//	D  target object id
//	D  skill id
//	D  skill level
//	D  hit time
//	D  reuse delay
//	D,D,D  caster x, y, z
//	H  unknown list size (0)
//	H  ground-location list size (0)
//	D,D,D  target x, y, z
func BuildMagicSkillUse(casterObjectID, targetObjectID, skillID, skillLevel, hitTime, reuseDelay int32, x, y, z int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x48)
	w.WriteD(casterObjectID)
	w.WriteD(targetObjectID)
	w.WriteD(skillID)
	w.WriteD(skillLevel)
	w.WriteD(hitTime)
	w.WriteD(reuseDelay)
	// caster location
	w.WriteD(x)
	w.WriteD(y)
	w.WriteD(z)
	// unknown list (empty)
	w.WriteH(0)
	// ground-location list (empty)
	w.WriteH(0)
	// target location (self-cast for shots → caster position)
	w.WriteD(x)
	w.WriteD(y)
	w.WriteD(z)
	return w.Bytes()
}
