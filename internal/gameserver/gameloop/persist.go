package gameloop

import (
	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// SetPersistSink wires the channel that receives character snapshots for async
// persistence. The game loop only ever sends value copies here (never the live
// pointer), so the receiving goroutine can write to the DB without racing the
// loop's unlocked mutation of player.Character.
func (gl *GameLoop) SetPersistSink(sink chan<- models.Character) {
	gl.persistSink = sink
}

// persistPlayer enqueues a value-copy snapshot of a player's character to the
// async saver. Non-blocking: if the sink is unset or full the snapshot is dropped
// (the next autosave tick or save-on-shutdown will capture the latest state).
//
// MUST be called on the game-loop goroutine — it reads live Character progress
// fields (EXP/SP/level/HP) that the loop mutates without a lock.
func (gl *GameLoop) persistPlayer(player *registry.PlayerWorldState) {
	if gl.persistSink == nil {
		return
	}
	snap, ok := player.SnapshotCharacter()
	if !ok {
		return
	}
	select {
	case gl.persistSink <- snap:
	default:
		log.Warn().Int32("char_id", player.CharID).Msg("persist sink full, dropping character snapshot")
	}
}

// autosaveOnlinePlayers snapshots every online player to the async saver. Runs on
// the game-loop goroutine (driven by the autosave timer in Run).
func (gl *GameLoop) autosaveOnlinePlayers() {
	players := gl.world.SnapshotPlayers(nil) // every 5 min; a fresh slice is fine (l2go-3rx)
	if len(players) == 0 {
		return
	}
	for _, player := range players {
		gl.persistPlayer(player)
	}
	log.Debug().Int("count", len(players)).Msg("autosave: enqueued online players for persistence")
}
