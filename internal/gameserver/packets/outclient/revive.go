package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildRevive builds the Revive packet (0x01, per L2J HF). Clears the death state on
// the client for the given object (broadcast to the revived character and everyone who
// sees it). Mirrors L2J Revive.writeImpl: opcode + objectId.
func BuildRevive(objectID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x01) // Revive opcode
	w.WriteD(objectID)
	return w.Bytes()
}
