package outclient

import (
	"bytes"
	"testing"
)

// ShowMiniMap (0xA3, HF): writeC(0xA3), writeD(mapId), writeC(SevenSigns period=0).
// Байты сверены с L2J-референсом serverpackets/ShowMiniMap.java.
func TestBuildShowMiniMap(t *testing.T) {
	got := BuildShowMiniMap(1665)
	// mapId=1665 = 0x0681 → LE: 81 06 00 00; период Seven Signs у нас всегда 0.
	want := []byte{0xA3, 0x81, 0x06, 0x00, 0x00, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("BuildShowMiniMap(1665) bytes mismatch\n got: % x\nwant: % x", got, want)
	}
}
