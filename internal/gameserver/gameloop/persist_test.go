package gameloop

import (
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

func TestGameLoop_AutosaveOnlinePlayers(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)

	// Give the player some progress and a non-origin position.
	player.Character.Experience = 4242
	player.Character.Level = 5
	player.Position = models.Position{X: 11, Y: 22, Z: 33}
	player.Heading = 180

	sink := make(chan models.Character, 4)
	gl.SetPersistSink(sink)

	gl.autosaveOnlinePlayers()

	select {
	case snap := <-sink:
		if snap.ID != 7 {
			t.Errorf("snapshot ID = %d, want 7", snap.ID)
		}
		if snap.Experience != 4242 || snap.Level != 5 {
			t.Errorf("progress not captured: exp=%d level=%d", snap.Experience, snap.Level)
		}
		if snap.Position != (models.Position{X: 11, Y: 22, Z: 33}) || snap.Heading != 180 {
			t.Errorf("live position/heading not baked in: pos=%+v heading=%d", snap.Position, snap.Heading)
		}
	default:
		t.Fatal("expected a snapshot on the persist sink, got none")
	}
}

func TestGameLoop_PersistPlayer_NilSink(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	// No sink set — must be a safe no-op, not a panic.
	gl.persistPlayer(player)
}

func TestGameLoop_PersistPlayer_FullSinkDoesNotBlock(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)

	sink := make(chan models.Character, 1)
	sink <- models.Character{} // fill it
	gl.SetPersistSink(sink)

	done := make(chan struct{})
	go func() {
		gl.persistPlayer(player) // must drop, not block
		close(done)
	}()

	select {
	case <-done:
		// ok — returned without blocking
	case <-time.After(time.Second):
		t.Fatal("persistPlayer blocked on a full sink")
	}
}
