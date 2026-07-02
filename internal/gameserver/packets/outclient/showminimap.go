package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildShowMiniMap строит пакет ShowMiniMap (0xA3, per L2J HF).
// Отправляется в ответ на RequestShowMiniMap (0x6C), иначе окно карты не откроется.
// Формат (serverpackets/ShowMiniMap.java): writeC(0xA3), writeD(mapId),
// writeC(SevenSigns.getCurrentPeriod()). Seven Signs у нас не реализован →
// период всегда 0 (нейтральный период, карта отображается штатно).
func BuildShowMiniMap(mapID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0xA3)
	w.WriteD(mapID)
	w.WriteC(0) // SevenSigns period (не реализован)
	return w.Bytes()
}
