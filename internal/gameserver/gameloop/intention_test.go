package gameloop

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

func TestStopPointWithinReach(t *testing.T) {
	target := models.Position{X: 0, Y: 0, Z: 100}

	t.Run("from far on +X axis → point at reach distance on +X", func(t *testing.T) {
		from := models.Position{X: 1000, Y: 0, Z: 100}
		p := stopPointWithinReach(from, target, 80)
		if p.X != 80 || p.Y != 0 {
			t.Errorf("got %+v, want X=80 Y=0", p)
		}
	})

	t.Run("already within reach → returns from unchanged", func(t *testing.T) {
		from := models.Position{X: 50, Y: 0, Z: 100}
		p := stopPointWithinReach(from, target, 80)
		if p != from {
			t.Errorf("got %+v, want unchanged %+v", p, from)
		}
	})

	t.Run("diagonal → preserves direction, distance≈reach", func(t *testing.T) {
		from := models.Position{X: 300, Y: 400, Z: 100} // dist 500
		p := stopPointWithinReach(from, target, 100)
		// direction (0.6, 0.8) * 100 = (60, 80)
		if p.X != 60 || p.Y != 80 {
			t.Errorf("got %+v, want X=60 Y=80", p)
		}
	})
}

func TestSetAndClearIntention(t *testing.T) {
	gl := &GameLoop{aiState: make(map[int32]*PlayerAIState)}

	gl.setIntention(7, IntentionAttack, 1003)
	st := gl.aiState[7]
	if st == nil || st.Intention != IntentionAttack || st.TargetObjectID != 1003 {
		t.Fatalf("setIntention not stored: %+v", st)
	}

	// overwrite replaces previous intention/target
	gl.setIntention(7, IntentionInteract, 2004)
	if st.Intention != IntentionInteract || st.TargetObjectID != 2004 {
		t.Errorf("setIntention did not overwrite: %+v", st)
	}

	gl.clearIntention(7)
	if st.Intention != IntentionIdle || st.TargetObjectID != 0 {
		t.Errorf("clearIntention did not reset: %+v", st)
	}
}
