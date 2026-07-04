package registry

import (
	"context"
	"fmt"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// buildTargetingWorld adds n players, each targeting the next player (a ring), so
// every player has exactly one targeter — the realistic "some people have you
// selected" case. Returns the registry.
func buildTargetingWorld(tb testing.TB, n int) *WorldRegistry {
	tb.Helper()
	wr := NewWorldRegistry()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		id := int32(i + 1)
		char := &models.Character{ID: id, Name: fmt.Sprintf("P%d", id)}
		if err := wr.AddPlayer(ctx, char); err != nil {
			tb.Fatalf("AddPlayer: %v", err)
		}
	}
	for i := 0; i < n; i++ {
		targeter := int32(i + 1)
		target := int32((i+1)%n + 1) // ring: last targets first
		wr.SetPlayerTarget(targeter, target)
	}
	return wr
}

// oldSweepScanAll replicates the pre-l2go-45b broadcastToTargeters cost: for the
// given object, copy the whole player map and scan it for TargetID matches. Called
// per-player in regen, this was the O(N^2) mine.
func oldSweepScanAll(wr *WorldRegistry, objectID int32) int {
	hits := 0
	for _, p := range wr.GetAllPlayers() {
		if p.TargetID == objectID {
			hits++
		}
	}
	return hits
}

// TestGetPlayersTargeting_MatchesFullScan locks the index result to the brute-force
// scan it replaces, across a world where targets are set and moved.
func TestGetPlayersTargeting_MatchesFullScan(t *testing.T) {
	wr := buildTargetingWorld(t, 50)
	// Re-point a few targeters to make the distribution non-uniform.
	wr.SetPlayerTarget(3, 10)
	wr.SetPlayerTarget(7, 10)
	wr.SetPlayerTarget(9, 10)
	wr.SetPlayerTarget(1, 0) // clears

	for obj := int32(1); obj <= 50; obj++ {
		got := len(wr.GetPlayersTargeting(obj))
		want := oldSweepScanAll(wr, obj)
		if got != want {
			t.Errorf("object %d: index says %d targeters, full scan says %d", obj, got, want)
		}
	}
}

// BenchmarkTargeterSweep_Index is the new path: a full regen sweep (query targeters
// of every online player) via the reverse index. Cost tracks total targeters, i.e.
// O(N) here (one targeter each), so ns/op grows LINEARLY with N — not quadratically.
func BenchmarkTargeterSweep_Index(b *testing.B) {
	for _, n := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			wr := buildTargetingWorld(b, n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for id := int32(1); id <= int32(n); id++ {
					_ = wr.GetPlayersTargeting(id)
				}
			}
		})
	}
}

// BenchmarkTargeterSweep_ScanAll is the OLD path for contrast: the same full sweep
// via GetAllPlayers + scan per player. Each query is O(N), the sweep O(N^2), so
// ns/op explodes quadratically — this is what l2go-45b removes.
func BenchmarkTargeterSweep_ScanAll(b *testing.B) {
	for _, n := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			wr := buildTargetingWorld(b, n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for id := int32(1); id <= int32(n); id++ {
					_ = oldSweepScanAll(wr, id)
				}
			}
		})
	}
}
