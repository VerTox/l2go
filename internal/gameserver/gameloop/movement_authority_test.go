package gameloop

import "testing"

// serverDrivenMovement gates which movements the loop interpolates: only combat/
// interact approach (server needs distance to target). Ground walking (MoveTo) and
// idle are client-authoritative — interpolating them caused rubber-band. (l2go-2ax)
func TestServerDrivenMovement(t *testing.T) {
	cases := []struct {
		intention Intention
		want      bool
	}{
		{IntentionAttack, true},
		{IntentionInteract, true},
		{IntentionMoveTo, false},
		{IntentionIdle, false},
		{IntentionCast, false},
		{IntentionFollow, false},
	}
	for _, tc := range cases {
		if got := serverDrivenMovement(tc.intention); got != tc.want {
			t.Errorf("serverDrivenMovement(%v) = %v, want %v", tc.intention, got, tc.want)
		}
	}
}
