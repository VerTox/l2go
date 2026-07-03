package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// AbnormalBuff is one active effect shown in the buff bar.
type AbnormalBuff struct {
	DisplayID    int32
	DisplayLevel int32
	RemainSec    int32 // remaining seconds; -1 = infinite (toggle)
}

// BuildAbnormalStatusUpdate builds AbnormalStatusUpdate (opcode 0x85) — the active
// buff/debuff icons and timers. L2J AbnormalStatusUpdate.writeImpl:
//
//	C  0x85
//	H  count
//	per effect: D displayId, H displayLevel, D remaining seconds (-1 infinite)
func BuildAbnormalStatusUpdate(buffs []AbnormalBuff) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x85)
	w.WriteH(uint16(len(buffs)))
	for _, b := range buffs {
		w.WriteD(b.DisplayID)
		w.WriteH(uint16(b.DisplayLevel))
		w.WriteD(b.RemainSec)
	}
	return w.Bytes()
}
