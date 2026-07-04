package registry

import (
	"sort"
	"sync"
	"testing"
)

func sortedTargeters(ti *targetIndex, objectID int32) []int32 {
	out := ti.targetersOf(objectID)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func eqInt32(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTargetIndex_SetMoveClear(t *testing.T) {
	ti := newTargetIndex()

	// Two players target NPC 500; one targets NPC 600.
	ti.set(1, 500)
	ti.set(2, 500)
	ti.set(3, 600)

	if got := sortedTargeters(ti, 500); !eqInt32(got, []int32{1, 2}) {
		t.Fatalf("targeters(500) = %v, want [1 2]", got)
	}
	if got := sortedTargeters(ti, 600); !eqInt32(got, []int32{3}) {
		t.Fatalf("targeters(600) = %v, want [3]", got)
	}

	// Player 1 switches target 500 -> 600: must leave 500's set and join 600's.
	ti.set(1, 600)
	if got := sortedTargeters(ti, 500); !eqInt32(got, []int32{2}) {
		t.Fatalf("after switch, targeters(500) = %v, want [2]", got)
	}
	if got := sortedTargeters(ti, 600); !eqInt32(got, []int32{1, 3}) {
		t.Fatalf("after switch, targeters(600) = %v, want [1 3]", got)
	}

	// Player 2 clears target (0): 500's set empties and the key is dropped.
	ti.set(2, 0)
	if got := ti.targetersOf(500); got != nil {
		t.Fatalf("after clear, targeters(500) = %v, want nil", got)
	}
	if _, exists := ti.targeters[500]; exists {
		t.Errorf("empty target key 500 was not deleted")
	}
}

func TestTargetIndex_SetIsIdempotent(t *testing.T) {
	ti := newTargetIndex()
	ti.set(1, 500)
	ti.set(1, 500) // repeat — must not duplicate
	if got := sortedTargeters(ti, 500); !eqInt32(got, []int32{1}) {
		t.Fatalf("targeters(500) = %v, want [1] (no duplicate on repeat set)", got)
	}
}

func TestTargetIndex_DropTarget(t *testing.T) {
	ti := newTargetIndex()
	ti.set(1, 500)
	ti.set(2, 500)

	// NPC 500 despawns.
	ti.dropTarget(500)

	if got := ti.targetersOf(500); got != nil {
		t.Fatalf("after drop, targeters(500) = %v, want nil", got)
	}
	// The dangling current entries were cleared, so re-targeting elsewhere is clean.
	ti.set(1, 700)
	if got := sortedTargeters(ti, 700); !eqInt32(got, []int32{1}) {
		t.Fatalf("targeters(700) = %v, want [1]", got)
	}
	// 500 must not resurrect.
	if _, exists := ti.targeters[500]; exists {
		t.Errorf("target key 500 resurrected after drop")
	}
}

func TestTargetIndex_TargetersOfSnapshotIsIndependent(t *testing.T) {
	ti := newTargetIndex()
	ti.set(1, 500)
	snap := ti.targetersOf(500)
	// Mutating the index after the snapshot must not change the returned slice.
	ti.set(2, 500)
	if len(snap) != 1 || snap[0] != 1 {
		t.Fatalf("snapshot changed after later set: %v", snap)
	}
}

// TestTargetIndex_ConcurrentAccess drives set/dropTarget/targetersOf from many
// goroutines to prove the index is race-free under -race (connection goroutines
// write targets while the loop goroutine reads targeters).
func TestTargetIndex_ConcurrentAccess(t *testing.T) {
	ti := newTargetIndex()
	const writers = 16
	const iters = 500

	var wg sync.WaitGroup
	// Writers: each charID cycles its target across a small set of objects.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(charID int32) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				ti.set(charID, int32(500+(i%8)))
			}
			ti.set(charID, 0)
		}(int32(w + 1))
	}
	// Reader: mimics the loop broadcasting to targeters.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iters*2; i++ {
			for obj := int32(500); obj < 508; obj++ {
				_ = ti.targetersOf(obj)
			}
		}
	}()
	// Dropper: mimics NPC despawns.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			ti.dropTarget(int32(500 + (i % 8)))
		}
	}()
	wg.Wait()

	// After every writer cleared to 0, no targeter should remain.
	for obj := int32(500); obj < 508; obj++ {
		if got := ti.targetersOf(obj); got != nil {
			t.Errorf("targeters(%d) = %v after all writers cleared, want nil", obj, got)
		}
	}
}
