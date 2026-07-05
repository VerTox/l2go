package gameloop

import "testing"

// serverDrivenMovement gates which movements the loop interpolates: only combat/
// interact/cast approach (server needs distance to target). Ground walking (MoveTo)
// and idle are client-authoritative — interpolating them caused rubber-band.
// (l2go-2ax; cast approach added in l2go-bdb)
func TestServerDrivenMovement(t *testing.T) {
	cases := []struct {
		intention Intention
		want      bool
	}{
		{IntentionAttack, true},
		{IntentionInteract, true},
		{IntentionCast, true}, // out-of-range cast runs to the target (l2go-bdb)
		{IntentionMoveTo, false},
		{IntentionIdle, false},
		{IntentionFollow, false},
	}
	for _, tc := range cases {
		if got := serverDrivenMovement(tc.intention); got != tc.want {
			t.Errorf("serverDrivenMovement(%v) = %v, want %v", tc.intention, got, tc.want)
		}
	}
}
