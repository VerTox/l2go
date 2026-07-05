package gameloop

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// loadMapRegions best-effort loads the nearest-town respawn data; returns false if
// the datapack isn't reachable from the test's working directory.
func loadMapRegions() bool {
	if registry.GetMapRegionRegistry().IsLoaded() {
		return true
	}
	for _, dir := range []string{"../../../references/data/mapregion", "../../../data/mapregion"} {
		if registry.GetMapRegionRegistry().LoadFromDirectory(dir) == nil {
			return true
		}
	}
	return registry.GetMapRegionRegistry().IsLoaded()
}

// TestApplyEscape_TeleportsToTown verifies Scroll of Escape relocates the caster to
// the region's respawn town (l2go-kg9), the same lookup death-respawn uses.
func TestApplyEscape_TeleportsToTown(t *testing.T) {
	if !loadMapRegions() {
		t.Skip("map region data not available in test environment")
	}
	gl, player := newTestLoopWithPlayer(t)
	player.Position = models.Position{X: 82000, Y: 148000, Z: -3400} // out in the field near Giran
	start := player.Position

	want, ok := registry.GetMapRegionRegistry().GetRespawnPoint(start.X, start.Y)
	if !ok {
		t.Fatalf("no respawn point resolved for %+v", start)
	}

	gl.applyEscape(player, "TOWN")

	if player.Position == start {
		t.Fatal("applyEscape did not move the player")
	}
	if player.Position.X != want.X || player.Position.Y != want.Y {
		t.Errorf("teleport dest X/Y = (%d,%d), want respawn (%d,%d)",
			player.Position.X, player.Position.Y, want.X, want.Y)
	}
}

// TestApplyEscape_UnsupportedTypeNoOp: non-TOWN escapes (clan hall/castle) aren't
// modelled yet and must not move the player.
func TestApplyEscape_UnsupportedTypeNoOp(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	start := player.Position
	gl.applyEscape(player, "CASTLE")
	if player.Position != start {
		t.Errorf("unsupported escape type moved the player: %+v -> %+v", start, player.Position)
	}
}
