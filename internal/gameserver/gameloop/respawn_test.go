package gameloop

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// TestHandleTeleportRelocatesAndFlags verifies the game-loop teleport primitive:
// the player is moved to the destination (with the L2J z+5 nudge), flagged teleporting,
// and its action/movement/target are aborted. (l2go-3xh.2)
func TestHandleTeleportRelocatesAndFlags(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	player.Position = models.Position{X: 1000, Y: 2000, Z: 100}
	player.TargetID = 555
	player.IsMoving = true

	gl.handleTeleport(CmdTeleport{
		CharID:  7,
		Dest:    models.Position{X: -84318, Y: 244579, Z: -3730},
		Heading: 16384,
	})

	if !player.IsTeleporting {
		t.Error("expected IsTeleporting=true after teleport")
	}
	wantZ := -3730 + teleportZOffset
	if player.Position.X != -84318 || player.Position.Y != 244579 || player.Position.Z != wantZ {
		t.Errorf("position = %+v, want {-84318, 244579, %d}", player.Position, wantZ)
	}
	if player.Heading != 16384 {
		t.Errorf("heading = %d, want 16384", player.Heading)
	}
	if player.TargetID != 0 {
		t.Error("target must be cleared on teleport")
	}
	if player.IsMoving {
		t.Error("server-side movement must stop on teleport")
	}
}

// TestRegisterWorldSpawnsPopulatesSpawnInfo verifies that spawn data is registered
// for NPCs already loaded into the world, so RespawnEvent can find it. Without this
// npcSpawnInfo is empty at startup and no NPC ever respawns ('spawn info not found').
// (l2go-c44)
func TestRegisterWorldSpawnsPopulatesSpawnInfo(t *testing.T) {
	world := registry.NewWorldRegistry()
	world.AddNPC(&models.NpcInstance{
		ObjectID:   1000,
		TemplateID: 42,
		Position:   models.Position{X: 10, Y: 20, Z: 30},
		Heading:    5,
		Template:   &models.NpcTemplate{ID: 42},
	})
	gl := New(world, registry.NewConnectionRegistry(), 1, 1)

	gl.RegisterWorldSpawns()

	info, ok := gl.npcSpawnInfo[1000]
	if !ok {
		t.Fatal("RegisterWorldSpawns did not register spawn info for a world NPC")
	}
	if info.TemplateID != 42 ||
		info.Position != (models.Position{X: 10, Y: 20, Z: 30}) ||
		info.Heading != 5 {
		t.Errorf("spawn info = %+v, want TemplateID 42, pos {10,20,30}, heading 5", info)
	}
}
