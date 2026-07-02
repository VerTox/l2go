package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildExUseSharedGroupItem builds the ExUseSharedGroupItem extended packet
// (0xFE:0x4A, per L2J HF serverpackets/ExUseSharedGroupItem.java). It syncs a
// consumable's reuse cooldown to the client so the item icon shows the reuse
// sweep (both when a use is refused during cooldown and when a use arms one).
//
// remainingSec / totalSec are in SECONDS: L2J divides the millisecond values by
// 1000 in the packet constructor.
//
// Format: writeC(0xFE), writeH(0x4A), writeD(itemId), writeD(groupId),
// writeD(remainingSec), writeD(totalSec).
func BuildExUseSharedGroupItem(itemID, groupID, remainingSec, totalSec int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0xfe)
	w.WriteH(0x4a)
	w.WriteD(itemID)
	w.WriteD(groupID)
	w.WriteD(remainingSec)
	w.WriteD(totalSec)
	return w.Bytes()
}
