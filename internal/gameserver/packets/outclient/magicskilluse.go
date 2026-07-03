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
//
// The final target location (L2J writeLoc(_target)) MUST be the target's real
// position, not the caster's — otherwise the client snaps the target to the caster
// (a mob "teleporting" to the caster on a ranged cast). For self-casts (shots,
// self-buffs) pass the caster position for both.
func BuildMagicSkillUse(casterObjectID, targetObjectID, skillID, skillLevel, hitTime, reuseDelay int32, cx, cy, cz, tx, ty, tz int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x48)
	w.WriteD(casterObjectID)
	w.WriteD(targetObjectID)
	w.WriteD(skillID)
	w.WriteD(skillLevel)
	w.WriteD(hitTime)
	w.WriteD(reuseDelay)
	// caster location
	w.WriteD(cx)
	w.WriteD(cy)
	w.WriteD(cz)
	// unknown list (empty)
	w.WriteH(0)
	// ground-location list (empty)
	w.WriteH(0)
	// target location
	w.WriteD(tx)
	w.WriteD(ty)
	w.WriteD(tz)
	return w.Bytes()
}
