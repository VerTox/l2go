package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildMyTargetSelected builds the MyTargetSelected packet (0xB9).
// Sent to the player to confirm they have selected a target.
func BuildMyTargetSelected(objectID int32, levelDiffColor int16) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0xB9)
	w.WriteD(objectID)        // target object ID
	w.WriteH(uint16(levelDiffColor)) // color based on level difference
	w.WriteD(0)               // reserved
	return w.Bytes()
}
