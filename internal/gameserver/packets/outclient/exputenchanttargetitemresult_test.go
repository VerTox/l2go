package outclient

import (
	"bytes"
	"testing"
)

// ExPutEnchantTargetItemResult (0xFE:0x81, HF): writeC(0xFE), writeH(0x81), writeD(result).
// Bytes verified against L2J serverpackets/ExPutEnchantTargetItemResult.java.
func TestBuildExPutEnchantTargetItemResult(t *testing.T) {
	// Success: result = target object id 0x1234 -> LE 34 12 00 00.
	got := BuildExPutEnchantTargetItemResult(0x1234)
	want := []byte{0xfe, 0x81, 0x00, 0x34, 0x12, 0x00, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("success bytes mismatch\n got: % x\nwant: % x", got, want)
	}

	// Failure: result = 0 -> LE 00 00 00 00.
	gotFail := BuildExPutEnchantTargetItemResult(0)
	wantFail := []byte{0xfe, 0x81, 0x00, 0x00, 0x00, 0x00, 0x00}
	if !bytes.Equal(gotFail, wantFail) {
		t.Errorf("failure bytes mismatch\n got: % x\nwant: % x", gotFail, wantFail)
	}
}
