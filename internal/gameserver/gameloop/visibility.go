package gameloop

import (
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// charInfoBuildCount counts BuildCharInfo passes for tests to assert the mover is
// built once per reconcile rather than once per observer (l2go-795). Loop-goroutine
// only, so no synchronization; a single int add is negligible in production.
var charInfoBuildCount int

// buildPlayerCharInfo builds a CharInfo packet for a player from live world state,
// using the cached paperdoll (no DB lookup) so it is safe to call every tick.
func buildPlayerCharInfo(player *registry.PlayerWorldState) []byte {
	charInfoBuildCount++
	char := player.Character
	ci := outclient.NewCharInfo(
		char,
		&player.Position,
		char.PaperdollItems,
		player.IsRunning,
		player.InCombat,
		player.Heading,
	)
	return ci.GetData()
}

// sendToPlayer sends packet data to a player's client connection if connected.
// Best-effort: a missing connection is a no-op (visibility state is still updated
// by the caller, so a transient gap does not wedge the known-set).
func (gl *GameLoop) sendToPlayer(player *registry.PlayerWorldState, data []byte) {
	if conn := gl.connections.GetConnection(player.AccountName); conn != nil {
		_ = conn.Send(data)
	}
}

// spawnPlayerTo shows `spawned` to `viewer`: CharInfo plus a RelationChanged so the
// client renders a normal (non-attackable) cursor instead of the sword cursor.
func (gl *GameLoop) spawnPlayerTo(viewer, spawned *registry.PlayerWorldState) {
	gl.sendToPlayer(viewer, buildPlayerCharInfo(spawned))
	relation := outclient.NewSingleRelation(spawned.CharID, int32(spawned.Character.Karma), 0)
	gl.sendToPlayer(viewer, relation.GetData())
}

// reconcileAllVisibility rebuilds player-to-player visibility for every online
// player as one bounded pass, decoupled from movement (l2go-awy). Mirrors L2J's
// KnownListUpdateTaskManager: instead of an O(local) reconcile on every movement
// step — O(N^2)/s when a crowd all moves — a single pass runs every
// visibilityInterval. reconcilePlayerVisibility is bidirectional and idempotent, so
// reconciling each player once covers spawn AND despawn for both sides. Loop-
// goroutine only (KnownPlayers is loop-owned). A fresh snapshot (not the shared
// playerScratch) since it runs ~1/s and calls back into range queries per player.
func (gl *GameLoop) reconcileAllVisibility() {
	for _, p := range gl.world.SnapshotPlayers(nil) {
		gl.reconcilePlayerVisibility(p.CharID)
	}
}

// reconcilePlayerVisibility brings the moving player's player-to-player visibility
// up to date: spawns (CharInfo) players newly in range and despawns (DeleteObject)
// players that left range — bidirectionally, so a stationary player also sees the
// mover appear/disappear. Runs only on the game-loop goroutine, which is the sole
// owner of every player's KnownPlayers set, so no locking is required. (l2go-23g)
func (gl *GameLoop) reconcilePlayerVisibility(charID int32) {
	mover, ok := gl.world.GetPlayer(charID)
	if !ok {
		return
	}

	// Keep-set: everyone within the (larger) forget radius stays spawned. Spawning,
	// though, only happens within the (smaller) watch radius — the gap between the two
	// is L2J's hysteresis band that prevents spawn/despawn flicker at the boundary.
	keep := make(map[int32]bool)
	for _, other := range gl.world.GetPlayersInRange(mover.Position, registry.VisibilityForgetRadius) {
		if other.CharID != charID {
			keep[other.CharID] = true
		}
	}

	// The mover's own spawn packets (CharInfo + Relation) are identical for every
	// observer, so build them once and reuse for all — previously spawnPlayerTo(other,
	// mover) rebuilt the mover's CharInfo once per observer, i.e. N times per reconcile
	// during a mass spawn (the O(N^2) the whole crowd pays). Built lazily so a reconcile
	// that spawns nobody new pays nothing. Reuse is safe: conn.Send copies the bytes
	// before its in-place XOR, so the shared slice is never mutated. (l2go-795)
	var moverCharInfo, moverRelation []byte

	// Entering range (within watch): spawn each side to the other exactly once.
	for _, other := range gl.world.GetPlayersInRange(mover.Position, registry.VisibilityWatchRadius) {
		if other.CharID == charID {
			continue
		}
		if !mover.KnownPlayers[other.CharID] {
			gl.spawnPlayerTo(mover, other)
			mover.KnownPlayers[other.CharID] = true
		}
		if !other.KnownPlayers[charID] {
			if moverCharInfo == nil {
				moverCharInfo = buildPlayerCharInfo(mover)
				moverRelation = outclient.NewSingleRelation(mover.CharID, int32(mover.Character.Karma), 0).GetData()
			}
			gl.sendToPlayer(other, moverCharInfo)
			gl.sendToPlayer(other, moverRelation)
			other.KnownPlayers[charID] = true
		}
	}

	// Leaving range (beyond forget): despawn each side from the other.
	for id := range mover.KnownPlayers {
		if keep[id] {
			continue
		}
		gl.sendToPlayer(mover, outclient.BuildDeleteObject(id))
		delete(mover.KnownPlayers, id)

		if other, ok := gl.world.GetPlayer(id); ok {
			gl.sendToPlayer(other, outclient.BuildDeleteObject(charID))
			delete(other.KnownPlayers, charID)
		}
	}
}

// despawnPlayerFromAll sends DeleteObject for a leaving player to everyone who had
// them spawned, and clears them from those known-sets. Called when a player
// disconnects so a later reconnect (same charID) is spawned fresh rather than being
// suppressed as "already known". (l2go-23g)
func (gl *GameLoop) despawnPlayerFromAll(charID int32) {
	data := outclient.BuildDeleteObject(charID)
	// Fresh slice (not the shared scratch): this runs during disconnect handling,
	// not a tick sweep, and is not hot enough to warrant buffer reuse. (l2go-3rx)
	for _, p := range gl.world.SnapshotPlayers(nil) {
		if p.CharID == charID {
			continue
		}
		if p.KnownPlayers[charID] {
			gl.sendToPlayer(p, data)
			delete(p.KnownPlayers, charID)
		}
	}
}
