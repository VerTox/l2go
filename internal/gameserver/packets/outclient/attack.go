package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// Attack hit flags, byte-for-byte from L2J HF Hit.java. The soulshot flag is
// OR'd with the weapon grade id (getItemGradeSPlus) in the same byte.
const (
	AttackFlagSS     = 0x10 // HITFLAG_USESS (| grade id)
	AttackFlagCrit   = 0x20 // HITFLAG_CRIT
	AttackFlagShield = 0x40 // HITFLAG_SHLD
	AttackFlagMiss   = 0x80 // HITFLAG_MISS
)

// BuildAttack builds the Attack packet (0x33).
// Sent to animate a melee attack between attacker and target.
// Based on L2J Attack.java.
//
// Format: attackerObjID, targetObjID, damage, flags, attackerXYZ,
// hitCount=0 (no extra hits), targetXYZ.
func BuildAttack(attackerObjID, targetObjID, damage, flags int32,
	atkX, atkY, atkZ, tgtX, tgtY, tgtZ int32) []byte {

	w := l2pkt.NewWriter()
	w.WriteC(0x33) // Attack opcode
	w.WriteD(attackerObjID)
	w.WriteD(targetObjID)
	w.WriteD(damage)
	w.WriteC(byte(flags))
	w.WriteD(atkX)
	w.WriteD(atkY)
	w.WriteD(atkZ)

	w.WriteH(0) // hit count (0 = single target, no extra hits)

	w.WriteD(tgtX)
	w.WriteD(tgtY)
	w.WriteD(tgtZ)

	return w.Bytes()
}
