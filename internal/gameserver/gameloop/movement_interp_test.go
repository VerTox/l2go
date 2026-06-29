package gameloop

import (
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

func TestInterpolatePosition(t *testing.T) {
	start := models.Position{X: 0, Y: 0, Z: 0}
	dest := models.Position{X: 100, Y: 200, Z: 0}

	t.Run("halfway", func(t *testing.T) {
		pos, arrived := interpolatePosition(start, dest, 500*time.Millisecond, time.Second)
		if arrived {
			t.Fatal("halfway should not be arrived")
		}
		if pos.X != 50 || pos.Y != 100 {
			t.Errorf("got %+v, want X=50 Y=100", pos)
		}
	})

	t.Run("elapsed >= total → arrived at dest", func(t *testing.T) {
		pos, arrived := interpolatePosition(start, dest, time.Second, time.Second)
		if !arrived || pos != dest {
			t.Errorf("got pos=%+v arrived=%v, want dest arrived", pos, arrived)
		}
	})

	t.Run("total <= 0 → arrived", func(t *testing.T) {
		_, arrived := interpolatePosition(start, dest, 0, 0)
		if !arrived {
			t.Error("zero total should report arrived")
		}
	})

	t.Run("elapsed <= 0 → at start", func(t *testing.T) {
		pos, arrived := interpolatePosition(start, dest, 0, time.Second)
		if arrived || pos != start {
			t.Errorf("got pos=%+v arrived=%v, want start not-arrived", pos, arrived)
		}
	})
}

func TestDistanceBetween(t *testing.T) {
	d := distanceBetween(models.Position{X: 0, Y: 0}, models.Position{X: 3, Y: 4})
	if d != 5 {
		t.Errorf("got %g, want 5", d)
	}
}
