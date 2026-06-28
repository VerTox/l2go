package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// Attack packet flags.
const (
	AttackFlagMiss   = 0x01
	AttackFlagCrit   = 0x20
	AttackFlagShield = 0x02
	AttackFlagSS     = 0x04
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
