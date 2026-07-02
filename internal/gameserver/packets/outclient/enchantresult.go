package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildEnchantResult builds the EnchantResult packet (opcode 0x87, per L2J HF
// serverpackets/EnchantResult.java). Sent after RequestEnchantItem to tell the
// client how the enchant attempt resolved.
//
// result codes (RequestEnchantItem.java):
//
//	0 = success (item enchant +1)
//	1 = failure, item destroyed, crystals returned (crystal id + count)
//	2 = error / invalid conditions
//	3 = blessed failure (enchant reset to 0)
//	4 = failure, item destroyed, no crystals
//	5 = safe failure (enchant unchanged)
//
// Format: writeC(0x87), writeD(result), writeD(crystalId), writeQ(crystalCount).
func BuildEnchantResult(result int32, crystalID int32, crystalCount int64) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x87)
	w.WriteD(result)
	w.WriteD(crystalID)
	w.WriteQ(crystalCount)
	return w.Bytes()
}
