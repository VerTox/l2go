package inclient

import "testing"

func TestParseRequestRestartPoint(t *testing.T) {
	// int32 LE = 0 (to nearest town)
	pkt, err := ParseRequestRestartPoint([]byte{0x00, 0x00, 0x00, 0x00})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pkt.RequestedPointType != RestartPointTown {
		t.Errorf("RequestedPointType = %d, want %d", pkt.RequestedPointType, RestartPointTown)
	}

	// non-zero type parses through (e.g. 2 = to castle)
	pkt2, err := ParseRequestRestartPoint([]byte{0x02, 0x00, 0x00, 0x00})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pkt2.RequestedPointType != 2 {
		t.Errorf("RequestedPointType = %d, want 2", pkt2.RequestedPointType)
	}
}
