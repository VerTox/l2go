package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildChooseInventoryItem builds the ChooseInventoryItem packet (opcode 0x7c,
// per L2J HF serverpackets/ChooseInventoryItem.java). Sent after a player uses an
// enchant scroll: it opens the client's "select an item to enchant" window,
// filtered to items compatible with the given scroll item id.
// Format: writeC(0x7c), writeD(itemId).
func BuildChooseInventoryItem(itemID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x7c)
	w.WriteD(itemID)
	return w.Bytes()
}
