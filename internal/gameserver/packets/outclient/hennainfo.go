package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

type HennaInfo struct {
	INT, STR, CON, MEN, DEX, WIT int32
	Slots                        [3]int32
}

func (h HennaInfo) Write(w *l2pkt.Writer) {
	w.WriteC(0xe5)
	w.WriteC(byte(h.INT))
	w.WriteC(byte(h.STR))
	w.WriteC(byte(h.CON))
	w.WriteC(byte(h.MEN))
	w.WriteC(byte(h.DEX))
	w.WriteC(byte(h.WIT))
	w.WriteD(3)
	w.WriteD(0) //size
}
