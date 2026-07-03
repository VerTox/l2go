package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildExAutoSoulShot builds ExAutoSoulShot (0xFE:0x0C): the server echo confirming
// a shot item was toggled for auto-use. type: 1 = on, 0 = off. Mirrors L2J HF
// ExAutoSoulShot.writeImpl.
func BuildExAutoSoulShot(itemID, typ int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0xFE)
	w.WriteH(0x0C)
	w.WriteD(itemID)
	w.WriteD(typ)
	return w.Bytes()
}
