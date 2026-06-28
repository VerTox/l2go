package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildTargetUnselected builds the TargetUnselected packet (0x24).
// Sent to confirm target deselection.
func BuildTargetUnselected(objectID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x24)
	w.WriteD(objectID) // character object ID
	w.WriteD(0)        // x (unused in most cases)
	w.WriteD(0)        // y
	w.WriteD(0)        // z
	w.WriteD(0)        // unknown
	return w.Bytes()
}
