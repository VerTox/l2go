package gameloop

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

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
