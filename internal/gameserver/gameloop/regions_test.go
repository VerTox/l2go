package gameloop

import (
	"testing"
	"time"
)

// TestRegions_NoLeakAfterWalkAndLeave is the l2go-wdl regression: a player that
// walks across many cells and then disconnects must leave every region's
// PlayerCount back at 0, so the stale sweep reclaims them — previously the count
// inflated on every move and never returned to 0, leaking activeRegions forever.
func TestRegions_NoLeakAfterWalkAndLeave(t *testing.T) {
	gl, _ := newTestLoopWithPlayer(t) // activeRegions starts empty (entry is command-driven)

	// Enter, then walk a long line crossing many cell boundaries.
	gl.updatePlayerRegions(100, 500, 500)
	for x := 500; x <= 20000; x += 400 {
		gl.updatePlayerRegions(100, x, 500)
	}

	// While present, the current block is active (>0 somewhere).
	if len(gl.activeRegions) == 0 {
		t.Fatal("expected some active regions while the player is in the world")
	}

	gl.leavePlayerRegions(100)

	// Every region must now be vacant.
	for key, r := range gl.activeRegions {
		if r.PlayerCount != 0 {
			t.Errorf("cell %s PlayerCount=%d after the only player left, want 0", key, r.PlayerCount)
		}
	}

	// Age them past the timeout and sweep — activeRegions must fully drain (no leak).
	past := time.Now().Add(-2 * regionDeactivateTimeout)
	for _, r := range gl.activeRegions {
		r.LastPlayerTime = past
	}
	gl.deactivateStaleRegions()
	if n := len(gl.activeRegions); n != 0 {
		t.Errorf("after stale sweep %d regions remain, want 0 — activeRegions leaks", n)
	}
}

// TestRegions_SameCellDoesNotInflate pins that repeated position updates within one
// cell do not increment PlayerCount (the root of the old leak).
func TestRegions_SameCellDoesNotInflate(t *testing.T) {
	gl, _ := newTestLoopWithPlayer(t)
	gl.updatePlayerRegions(100, 500, 500) // cell (0,0)
	for i := 0; i < 200; i++ {
		gl.updatePlayerRegions(100, 500+i, 500) // stays in cell (0,0) for i < 500
	}
	center := gl.activeRegions[cellKey([2]int{0, 0})]
	if center == nil || center.PlayerCount != 1 {
		t.Fatalf("same-cell moves inflated PlayerCount: %+v", center)
	}
}

// TestRegions_RefCountMultiplePlayers checks the count tracks real occupancy.
func TestRegions_RefCountMultiplePlayers(t *testing.T) {
	gl, _ := newTestLoopWithPlayer(t)
	gl.updatePlayerRegions(100, 500, 500)
	gl.updatePlayerRegions(200, 500, 500) // same cell
	center := gl.activeRegions[cellKey([2]int{0, 0})]
	if center.PlayerCount != 2 {
		t.Fatalf("two players in a cell: PlayerCount=%d, want 2", center.PlayerCount)
	}
	gl.leavePlayerRegions(100)
	if center.PlayerCount != 1 {
		t.Fatalf("after one left: PlayerCount=%d, want 1", center.PlayerCount)
	}
	gl.leavePlayerRegions(200)
	if center.PlayerCount != 0 {
		t.Fatalf("after both left: PlayerCount=%d, want 0", center.PlayerCount)
	}
}

// TestRegions_StillOccupiedNotReclaimed guards the timeout: a region a player still
// stands in must NOT be swept even if LastPlayerTime is old (PlayerCount>0 protects
// it), so a stationary player is never deactivated out from under.
func TestRegions_StillOccupiedNotReclaimed(t *testing.T) {
	gl, _ := newTestLoopWithPlayer(t)
	gl.updatePlayerRegions(100, 500, 500)
	for _, r := range gl.activeRegions {
		r.LastPlayerTime = time.Now().Add(-time.Hour) // ancient, but still occupied
	}
	gl.deactivateStaleRegions()
	if len(gl.activeRegions) != 9 {
		t.Errorf("occupied regions were reclaimed: %d remain, want 9", len(gl.activeRegions))
	}
}
