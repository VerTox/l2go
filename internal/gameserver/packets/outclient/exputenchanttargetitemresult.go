package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildExPutEnchantTargetItemResult builds the ExPutEnchantTargetItemResult
// extended packet (0xFE:0x81, per L2J HF serverpackets/ExPutEnchantTargetItemResult.java).
// Sent in reply to RequestExTryToPutEnchantTargetItem in the windowed enchant flow:
// result is the target item's object id on success (the item appears in the
// enchant window) or 0 on failure (the client keeps the window empty).
//
// Format: writeC(0xFE), writeH(0x81), writeD(result).
func BuildExPutEnchantTargetItemResult(result int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0xfe)
	w.WriteH(0x81)
	w.WriteD(result)
	return w.Bytes()
}
