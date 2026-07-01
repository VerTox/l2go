package registry

import "github.com/VerTox/l2go/internal/gameserver/models"

// SnapshotCharacter returns a value copy of the player's Character with the live
// world Position/Heading baked in, suitable for off-goroutine persistence.
//
// The copy is what makes async saving race-free: the game loop mutates
// player.Character (EXP/SP/level/HP) without a lock, so the DB write must operate
// on a detached copy rather than the live pointer. models.Character holds only
// scalars, value arrays and a read-only *time.Time, so a shallow copy is safe.
//
// This reads live Character fields without locking, so it MUST be called either on
// the game-loop goroutine (the sole writer of progress fields) or under the
// registry lock (see SnapshotOnlineCharacters).
func (s *PlayerWorldState) SnapshotCharacter() (models.Character, bool) {
	if s.Character == nil {
		return models.Character{}, false
	}
	c := *s.Character
	c.Position = s.Position
	c.SetHeading(int(s.Heading))
	return c, true
}

// SnapshotOnlineCharacters returns value-copy snapshots of every online player's
// character, taken under the registry read lock. Used by save-on-shutdown after
// the game loop has stopped: the lock serializes with any lingering connection
// goroutines still writing position via UpdatePlayerPosition.
func (wr *WorldRegistry) SnapshotOnlineCharacters() []models.Character {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	out := make([]models.Character, 0, len(wr.players))
	for _, state := range wr.players {
		if snap, ok := state.SnapshotCharacter(); ok {
			out = append(out, snap)
		}
	}
	return out
}
