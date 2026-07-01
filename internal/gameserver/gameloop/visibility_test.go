package gameloop

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// addPlayer registers a second player in the loop's world at pos and returns its state.
func addPlayer(t *testing.T, gl *GameLoop, id int32, account string, pos models.Position) {
	t.Helper()
	char := &models.Character{ID: id, AccountName: account, Name: account, MaxHP: 100, CurrentHP: 100, Position: pos}
	if err := gl.world.AddPlayer(context.Background(), char); err != nil {
		t.Fatalf("AddPlayer %d: %v", id, err)
	}
}

func TestReconcilePlayerVisibility_BidirectionalSpawnAndDespawn(t *testing.T) {
	gl, p1 := newTestLoopWithPlayer(t) // charID 7 at origin
	ctx := context.Background()

	// Second player far out of range.
	addPlayer(t, gl, 8, "acc2", models.Position{X: 5000, Y: 0, Z: 0})
	p2, _ := gl.world.GetPlayer(8)

	// Out of range: reconciling either sees no one.
	gl.reconcilePlayerVisibility(7)
	if p1.KnownPlayers[8] || p2.KnownPlayers[7] {
		t.Fatalf("players out of range should not know each other: p1→p2=%v p2→p1=%v",
			p1.KnownPlayers[8], p2.KnownPlayers[7])
	}

	// P2 walks into P1's range. Reconciling the mover must spawn BOTH ways, so the
	// stationary P1 also gets P2 (this is the bug that was broken).
	_ = gl.world.UpdatePlayerPosition(ctx, 8, models.Position{X: 100, Y: 0, Z: 0}, 0)
	gl.reconcilePlayerVisibility(8)
	if !p2.KnownPlayers[7] {
		t.Error("mover P2 should now know stationary P1")
	}
	if !p1.KnownPlayers[8] {
		t.Error("stationary P1 should now know mover P2 (bidirectional spawn)")
	}

	// Reconciling again is idempotent — no duplicate bookkeeping churn.
	gl.reconcilePlayerVisibility(8)
	if !p2.KnownPlayers[7] || !p1.KnownPlayers[8] {
		t.Error("known-sets should remain stable on repeated reconcile")
	}

	// P2 walks back out of range: both sides forget each other.
	_ = gl.world.UpdatePlayerPosition(ctx, 8, models.Position{X: 5000, Y: 0, Z: 0}, 0)
	gl.reconcilePlayerVisibility(8)
	if p2.KnownPlayers[7] {
		t.Error("mover P2 should forget P1 after leaving range")
	}
	if p1.KnownPlayers[8] {
		t.Error("P1 should forget mover P2 after it left range (bidirectional despawn)")
	}
}

func TestDespawnPlayerFromAll(t *testing.T) {
	gl, p1 := newTestLoopWithPlayer(t)
	addPlayer(t, gl, 8, "acc2", models.Position{X: 100, Y: 0, Z: 0})
	p2, _ := gl.world.GetPlayer(8)

	// Establish mutual visibility.
	gl.reconcilePlayerVisibility(7)
	if !p1.KnownPlayers[8] || !p2.KnownPlayers[7] {
		t.Fatal("precondition: players should see each other")
	}

	// P2 disconnects — everyone who knew P2 forgets them.
	gl.despawnPlayerFromAll(8)
	if p1.KnownPlayers[8] {
		t.Error("P1 should forget disconnected P2 (else a reconnect stays invisible)")
	}
}
