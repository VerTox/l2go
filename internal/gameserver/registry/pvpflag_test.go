package registry

import (
	"testing"
	"time"
)

func TestIsPvPFlagged(t *testing.T) {
	now := time.Unix(1000, 0)
	p := &PlayerWorldState{}

	if p.IsPvPFlagged(now) {
		t.Fatal("zero PvPFlagUntil must not be flagged")
	}

	p.PvPFlagUntil = now.Add(10 * time.Second)
	if !p.IsPvPFlagged(now) {
		t.Fatal("future PvPFlagUntil must be flagged")
	}
	if p.IsPvPFlagged(now.Add(11 * time.Second)) {
		t.Fatal("expired PvPFlagUntil must not be flagged")
	}
}
