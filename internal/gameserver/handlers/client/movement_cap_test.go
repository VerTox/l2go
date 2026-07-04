package client

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

func playerAt(id int32, x, y int) *registry.PlayerWorldState {
	return &registry.PlayerWorldState{CharID: id, Position: models.Position{X: x, Y: y, Z: 0}}
}

func TestClosestPlayers_ReturnsNearestK(t *testing.T) {
	origin := models.Position{X: 0, Y: 0, Z: 0}
	// Distances from origin: id1=100, id2=200, id3=300, id4=400, id5=500.
	players := []*registry.PlayerWorldState{
		playerAt(3, 300, 0),
		playerAt(1, 100, 0),
		playerAt(5, 500, 0),
		playerAt(2, 200, 0),
		playerAt(4, 400, 0),
	}

	got := closestPlayers(players, origin, 3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	want := map[int32]bool{1: true, 2: true, 3: true} // the three nearest
	for _, p := range got {
		if !want[p.CharID] {
			t.Errorf("closestPlayers returned %d, which is not among the 3 nearest", p.CharID)
		}
	}
}

func TestClosestPlayers_NoCapWhenUnderK(t *testing.T) {
	players := []*registry.PlayerWorldState{playerAt(1, 100, 0), playerAt(2, 200, 0)}
	got := closestPlayers(players, models.Position{}, 48)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (below cap → all returned)", len(got))
	}
}

// TestSqDist_NoOverflowAtMapScale guards the int64 math: coordinates span roughly
// +/-150k, so a naive int32 dx*dx would overflow (300k^2 = 9e10 > 2.1e9).
func TestSqDist_NoOverflowAtMapScale(t *testing.T) {
	a := models.Position{X: -150000, Y: -150000}
	b := models.Position{X: 150000, Y: 150000}
	got := sqDist(a, b)
	want := int64(300000)*300000 + int64(300000)*300000 // 1.8e11
	if got != want {
		t.Fatalf("sqDist = %d, want %d (overflow?)", got, want)
	}
}

// TestClosestPlayers_PicksActualNearestInMixedField sanity-checks 2D ordering.
func TestClosestPlayers_PicksActualNearestInMixedField(t *testing.T) {
	pos := models.Position{X: 1000, Y: 1000}
	players := []*registry.PlayerWorldState{
		playerAt(1, 1050, 1050), // ~70
		playerAt(2, 5000, 5000), // far
		playerAt(3, 900, 1100),  // ~141
		playerAt(4, 8000, 100),  // far
	}
	got := closestPlayers(players, pos, 2)
	ids := map[int32]bool{}
	for _, p := range got {
		ids[p.CharID] = true
	}
	if !ids[1] || !ids[3] {
		t.Errorf("expected nearest ids {1,3}, got %v", ids)
	}
}
