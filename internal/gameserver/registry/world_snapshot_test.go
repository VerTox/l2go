package registry

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

func TestSnapshotPlayers_ReturnsAllLivePointers(t *testing.T) {
	wr := NewWorldRegistry()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := wr.AddPlayer(ctx, &models.Character{ID: int32(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}

	snap := wr.SnapshotPlayers(nil)
	if len(snap) != 5 {
		t.Fatalf("len(snap) = %d, want 5", len(snap))
	}
	// Pointers must be the live registry states, not copies (mutations visible).
	seen := map[int32]bool{}
	for _, p := range snap {
		live, ok := wr.GetPlayer(p.CharID)
		if !ok || live != p {
			t.Errorf("snapshot pointer for %d is not the live state", p.CharID)
		}
		seen[p.CharID] = true
	}
	for id := int32(1); id <= 5; id++ {
		if !seen[id] {
			t.Errorf("player %d missing from snapshot", id)
		}
	}
}

func TestSnapshotPlayers_ReusesBuffer(t *testing.T) {
	wr := NewWorldRegistry()
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		_ = wr.AddPlayer(ctx, &models.Character{ID: int32(i + 1)})
	}

	// First call sizes the buffer; subsequent calls must reuse its backing array.
	buf := wr.SnapshotPlayers(nil)
	first := &buf[:1][0]
	buf2 := wr.SnapshotPlayers(buf)
	if len(buf2) != 100 {
		t.Fatalf("len = %d, want 100", len(buf2))
	}
	if &buf2[:1][0] != first {
		t.Errorf("buffer was reallocated instead of reused")
	}

	// After warm-up, reusing the buffer must not allocate at all.
	allocs := testing.AllocsPerRun(100, func() {
		buf = wr.SnapshotPlayers(buf)
	})
	if allocs != 0 {
		t.Errorf("SnapshotPlayers with a reused buffer allocated %v times/op, want 0", allocs)
	}
}

// TestSnapshotPlayers_EquivalentToGetAllPlayers pins the snapshot to the map-copy
// it replaces on the loop's sweeps: same set of player pointers.
func TestSnapshotPlayers_EquivalentToGetAllPlayers(t *testing.T) {
	wr := NewWorldRegistry()
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		_ = wr.AddPlayer(ctx, &models.Character{ID: int32(i + 1)})
	}

	m := wr.GetAllPlayers()
	snap := wr.SnapshotPlayers(nil)
	if len(snap) != len(m) {
		t.Fatalf("snapshot len %d != map len %d", len(snap), len(m))
	}
	for _, p := range snap {
		if m[p.CharID] != p {
			t.Errorf("snapshot pointer for %d differs from GetAllPlayers", p.CharID)
		}
	}
}

func benchWorld(b *testing.B, n int) *WorldRegistry {
	b.Helper()
	wr := NewWorldRegistry()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		_ = wr.AddPlayer(ctx, &models.Character{ID: int32(i + 1)})
	}
	return wr
}

// BenchmarkGetAllPlayers is the old per-sweep cost: a fresh map every call.
func BenchmarkGetAllPlayers(b *testing.B) {
	wr := benchWorld(b, 1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = wr.GetAllPlayers()
	}
}

// BenchmarkSnapshotPlayersReuse is the new cost: a reused buffer → zero allocs.
func BenchmarkSnapshotPlayersReuse(b *testing.B) {
	wr := benchWorld(b, 1000)
	var buf []*PlayerWorldState
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = wr.SnapshotPlayers(buf)
	}
}
