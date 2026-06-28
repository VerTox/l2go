package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildAutoAttackStop builds the AutoAttackStop packet (0x26).
// Notifies the client that auto-attack has stopped.
func BuildAutoAttackStop(targetObjID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x26) // AutoAttackStop opcode
	w.WriteD(targetObjID)
	return w.Bytes()
}
