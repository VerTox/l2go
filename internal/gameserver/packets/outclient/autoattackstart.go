package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildAutoAttackStart builds the AutoAttackStart packet (0x25).
// Notifies the client that auto-attack has begun on the given target.
func BuildAutoAttackStart(targetObjID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x25) // AutoAttackStart opcode
	w.WriteD(targetObjID)
	return w.Bytes()
}
