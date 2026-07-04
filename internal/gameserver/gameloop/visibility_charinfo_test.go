package gameloop

import (
	"fmt"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// TestReconcile_MoverCharInfoBuiltOncePerReconcile pins the l2go-795 fix: the
// mover's CharInfo is built once per reconcile pass and reused for every observer,
// not rebuilt per observer (which was O(N) per reconcile → O(N^2) at a mass spawn).
//
// Setup isolates mover-builds: the mover already knows every observer (so the
// "spawn other to mover" branch, which builds each other, is skipped), while no
// observer knows the mover (so the "spawn mover to observer" branch runs for all).
// Every CharInfo build in the pass is therefore a mover build.
func TestReconcile_MoverCharInfoBuiltOncePerReconcile(t *testing.T) {
	for _, observers := range []int{1, 10, 100} {
		t.Run(fmt.Sprintf("observers=%d", observers), func(t *testing.T) {
			gl, mover := newTestLoopWithPlayer(t) // charID 7 at origin

			for i := 0; i < observers; i++ {
				id := int32(100 + i)
				addPlayer(t, gl, id, fmt.Sprintf("obs%d", id), models.Position{X: int(100 + i), Y: 0, Z: 0})
				// Mover already knows this observer → its CharInfo won't be built.
				mover.KnownPlayers[id] = true
				// Observer does NOT know the mover → the mover is spawned to it.
			}

			charInfoBuildCount = 0
			gl.reconcilePlayerVisibility(7)

			if charInfoBuildCount != 1 {
				t.Errorf("observers=%d: mover CharInfo built %d times, want exactly 1 (reused for all observers)",
					observers, charInfoBuildCount)
			}
			// All observers must now know the mover (they received its CharInfo).
			for i := 0; i < observers; i++ {
				id := int32(100 + i)
				other, _ := gl.world.GetPlayer(id)
				if !other.KnownPlayers[7] {
					t.Errorf("observer %d did not learn the mover", id)
				}
			}
		})
	}
}

// TestReconcile_StillSpawnsBothDirections guards that the reuse refactor did not
// break the bidirectional spawn: a mover entering a stranger's range makes both
// sides know each other.
func TestReconcile_StillSpawnsBothDirections(t *testing.T) {
	gl, mover := newTestLoopWithPlayer(t) // charID 7 at origin
	addPlayer(t, gl, 8, "acc8", models.Position{X: 100, Y: 0, Z: 0})
	other, _ := gl.world.GetPlayer(8)

	gl.reconcilePlayerVisibility(7)

	if !mover.KnownPlayers[8] {
		t.Error("mover did not learn the in-range other")
	}
	if !other.KnownPlayers[7] {
		t.Error("in-range other did not learn the mover")
	}
}
