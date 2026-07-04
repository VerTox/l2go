package gameloop

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// TestReconcileAllVisibility_EstablishesVisibility verifies the periodic pass
// (l2go-awy) spawns players that moved into range, replacing the per-move reconcile.
func TestReconcileAllVisibility_EstablishesVisibility(t *testing.T) {
	gl, p1 := newTestLoopWithPlayer(t) // charID 7 at origin
	addPlayer(t, gl, 8, "acc8", models.Position{X: 5000, Y: 0, Z: 0})
	p2, _ := gl.world.GetPlayer(8)

	// Move p2 into range with only a registry position sync — no reconcile.
	_ = gl.world.UpdatePlayerPosition(context.Background(), 8, models.Position{X: 100, Y: 0, Z: 0}, 0)
	if p1.KnownPlayers[8] || p2.KnownPlayers[7] {
		t.Fatal("visibility must not be established before the periodic pass runs")
	}

	gl.reconcileAllVisibility()

	if !p1.KnownPlayers[8] || !p2.KnownPlayers[7] {
		t.Errorf("periodic reconcile did not establish visibility both ways: p1→p2=%v p2→p1=%v",
			p1.KnownPlayers[8], p2.KnownPlayers[7])
	}
}

// TestHandlePlayerMoved_DoesNotReconcile pins the decoupling: a ValidatePosition
// move syncs the position but must NOT reconcile visibility (the periodic pass does).
func TestHandlePlayerMoved_DoesNotReconcile(t *testing.T) {
	gl, p1 := newTestLoopWithPlayer(t) // charID 7 at origin
	addPlayer(t, gl, 8, "acc8", models.Position{X: 100, Y: 0, Z: 0})
	p2, _ := gl.world.GetPlayer(8)

	gl.handlePlayerMoved(CmdPlayerMoved{CharID: 8, Position: models.Position{X: 150, Y: 0, Z: 0}})

	// Position synced...
	if p2.Position.X != 150 {
		t.Errorf("handlePlayerMoved did not sync position: X=%d, want 150", p2.Position.X)
	}
	// ...but visibility NOT reconciled (l2go-awy).
	if p1.KnownPlayers[8] || p2.KnownPlayers[7] {
		t.Errorf("handlePlayerMoved must not reconcile visibility: p1→p2=%v p2→p1=%v",
			p1.KnownPlayers[8], p2.KnownPlayers[7])
	}

	// The periodic pass then establishes it.
	gl.reconcileAllVisibility()
	if !p1.KnownPlayers[8] || !p2.KnownPlayers[7] {
		t.Error("periodic pass should establish visibility after the move")
	}
}

// TestHandlePlayerEnteredWorld_StillReconcilesImmediately guards that entry (login
// and teleport-arrival) keeps its immediate reconcile — it must NOT wait for the
// periodic pass, or a player would enter the world blind for up to a second.
func TestHandlePlayerEnteredWorld_StillReconcilesImmediately(t *testing.T) {
	gl, p1 := newTestLoopWithPlayer(t) // charID 7 at origin
	addPlayer(t, gl, 8, "acc8", models.Position{X: 100, Y: 0, Z: 0})
	p2, _ := gl.world.GetPlayer(8)

	gl.handlePlayerEnteredWorld(CmdPlayerEnteredWorld{
		CharID: 8, AccountName: "acc8", Position: models.Position{X: 100, Y: 0, Z: 0},
	})

	if !p2.KnownPlayers[7] || !p1.KnownPlayers[8] {
		t.Errorf("entry must reconcile immediately: newcomer→existing=%v existing→newcomer=%v",
			p2.KnownPlayers[7], p1.KnownPlayers[8])
	}
}
