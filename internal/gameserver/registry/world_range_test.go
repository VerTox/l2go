package registry

import (
	"context"
	"fmt"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// bruteForcePlayersInRange is the reference (pre-grid) semantics: a linear scan
// over every player. GetPlayersInRange must return the exact same set once it is
// backed by the region grid (l2go-g63).
func bruteForcePlayersInRange(wr *WorldRegistry, pos models.Position, radius int) map[int32]bool {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	out := make(map[int32]bool)
	for id, state := range wr.players {
		if wr.calculateDistance(pos, state.Position) <= radius {
			out[id] = true
		}
	}
	return out
}

func toIDSet(states []*PlayerWorldState) map[int32]bool {
	out := make(map[int32]bool, len(states))
	for _, s := range states {
		out[s.CharID] = true
	}
	return out
}

func sameIDSet(a, b map[int32]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// gridSide is the side length (in players) of the square field scatterPlayers lays
// out for n players, so a benchmark can aim a query at the field's dense center.
func gridSide(n int) int {
	side := 1
	for side*side < n {
		side++
	}
	return side
}

// scatterPlayers adds n players on a deterministic grid, `step` units apart,
// starting at the origin. With step > 1000 (region size) they land in distinct
// regions, exercising getNearbyRegions.
func scatterPlayers(t testing.TB, wr *WorldRegistry, n, step int) {
	t.Helper()
	ctx := context.Background()
	side := gridSide(n)
	added := 0
	for gy := 0; gy < side && added < n; gy++ {
		for gx := 0; gx < side && added < n; gx++ {
			id := int32(added + 1)
			char := &models.Character{ID: id, Name: fmt.Sprintf("P%d", id)}
			char.SetPosition(gx*step, gy*step, 0)
			if err := wr.AddPlayer(ctx, char); err != nil {
				t.Fatalf("AddPlayer(%d): %v", id, err)
			}
			added++
		}
	}
}

func TestGetPlayersInRange_MatchesBruteForce(t *testing.T) {
	wr := NewWorldRegistry()
	// 400 players spread 1500 units apart → many distinct regions, several
	// players share a region only at the coarse boundaries.
	scatterPlayers(t, wr, 400, 1500)

	cases := []struct {
		pos    models.Position
		radius int
	}{
		{models.Position{X: 0, Y: 0, Z: 0}, 100},        // corner, tiny radius
		{models.Position{X: 0, Y: 0, Z: 0}, 3400},       // corner, visibility radius
		{models.Position{X: 15000, Y: 15000, Z: 0}, 2000}, // middle of the field
		{models.Position{X: 999, Y: 999, Z: 0}, 1200},   // straddles a region boundary
		{models.Position{X: -5000, Y: -5000, Z: 0}, 500}, // empty area → nothing
		{models.Position{X: 30000, Y: 30000, Z: 0}, 50000}, // huge radius → everyone
	}

	for _, c := range cases {
		want := bruteForcePlayersInRange(wr, c.pos, c.radius)
		got := toIDSet(wr.GetPlayersInRange(c.pos, c.radius))
		if !sameIDSet(got, want) {
			t.Errorf("GetPlayersInRange(%+v, %d): got %d players, want %d (sets differ)",
				c.pos, c.radius, len(got), len(want))
		}
	}
}

// TestGetPlayersInRange_TracksMovement verifies the region index stays consistent
// with GetPlayersInRange after a player crosses a region boundary — a linear scan
// is trivially correct here, so this guards the grid path against index drift.
func TestGetPlayersInRange_TracksMovement(t *testing.T) {
	wr := NewWorldRegistry()
	ctx := context.Background()

	char := &models.Character{ID: 1, Name: "Mover"}
	char.SetPosition(0, 0, 0)
	if err := wr.AddPlayer(ctx, char); err != nil {
		t.Fatal(err)
	}

	q := models.Position{X: 50000, Y: 50000, Z: 0}
	if got := wr.GetPlayersInRange(q, 200); len(got) != 0 {
		t.Fatalf("before move: got %d, want 0", len(got))
	}

	// Move next to the far query point (crosses ~50 regions).
	if err := wr.UpdatePlayerPosition(ctx, 1, models.Position{X: 50100, Y: 50000, Z: 0}, 0); err != nil {
		t.Fatal(err)
	}

	got := toIDSet(wr.GetPlayersInRange(q, 200))
	if !got[1] {
		t.Fatalf("after move: player not found near new position, got %v", got)
	}
	// And it must no longer be found at the old region.
	if old := wr.GetPlayersInRange(models.Position{X: 0, Y: 0, Z: 0}, 200); len(old) != 0 {
		t.Fatalf("after move: still found at old position, got %d", len(old))
	}
}

// BenchmarkGetPlayersInRange measures a fixed-radius query as the total online
// count grows, under a NORMAL (spread) distribution — players ~1500 units apart,
// so a 3400-radius query only ever sees a handful regardless of total N. This is
// the scenario g63 targets: with the region grid, query cost tracks local density,
// not total online N, so ns/op stays flat as N grows (acceptance: time does not
// grow linearly with N). The pathological "everyone in one spot" case is out of
// scope here — it degrades to O(players-in-cell) by design and is addressed by
// scale #4/#5 (region-limited work + CharInfo dedup). Run: go test -bench
// GetPlayersInRange -run x.
func BenchmarkGetPlayersInRange(b *testing.B) {
	for _, n := range []int{100, 1000, 5000, 10000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			const step = 1500 // normal spread: ~1 player per region
			wr := NewWorldRegistry()
			scatterPlayers(b, wr, n, step)
			// Aim at the field's center so every N samples an equally-dense
			// neighborhood: this isolates "does total online N matter?" from
			// how many players happen to sit near the query point.
			mid := (gridSide(n) / 2) * step
			q := models.Position{X: mid, Y: mid, Z: 0}
			const radius = 3400 // visibility watch radius
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = wr.GetPlayersInRange(q, radius)
			}
		})
	}
}
