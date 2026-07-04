package registry

import "sync"

// targetIndex is a reverse index of who-is-targeting-what. It lets the game loop
// broadcast an object's updates (HP bar, status) to exactly the players aiming at
// that object in O(targeters) instead of scanning every online player via a full
// GetAllPlayers copy — the old broadcastToTargeters was O(N) per call and, called
// per-player each regen/DoT tick, O(N^2) overall. (l2go-45b)
//
// It self-tracks each targeter's current target (the `current` map), so callers
// only supply the NEW target — the previous one is looked up and unlinked here.
// Guarded by its own mutex, independent of the WorldRegistry lock, so target
// changes on connection goroutines never contend with world queries on the loop.
type targetIndex struct {
	mu        sync.RWMutex
	current   map[int32]int32              // targeter charID -> its current target objectID
	targeters map[int32]map[int32]struct{} // target objectID -> set of targeter charIDs
}

func newTargetIndex() *targetIndex {
	return &targetIndex{
		current:   make(map[int32]int32),
		targeters: make(map[int32]map[int32]struct{}),
	}
}

// set points targeter at newTarget (0 = no target), unlinking it from whatever it
// targeted before. Idempotent when the target is unchanged.
func (ti *targetIndex) set(targeterCharID, newTarget int32) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	old := ti.current[targeterCharID]
	if old == newTarget {
		return
	}

	if old != 0 {
		if s := ti.targeters[old]; s != nil {
			delete(s, targeterCharID)
			if len(s) == 0 {
				delete(ti.targeters, old)
			}
		}
	}

	if newTarget != 0 {
		s := ti.targeters[newTarget]
		if s == nil {
			s = make(map[int32]struct{})
			ti.targeters[newTarget] = s
		}
		s[targeterCharID] = struct{}{}
		ti.current[targeterCharID] = newTarget
	} else {
		delete(ti.current, targeterCharID)
	}
}

// dropTarget removes an object as a target entirely — used when the object leaves
// the world (player disconnect / NPC despawn), so its targeter set doesn't linger.
// The (now dangling) `current` entries of its targeters are cleared too; a later
// set() by any of them would self-heal regardless.
func (ti *targetIndex) dropTarget(objectID int32) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	s := ti.targeters[objectID]
	if s == nil {
		return
	}
	for charID := range s {
		if ti.current[charID] == objectID {
			delete(ti.current, charID)
		}
	}
	delete(ti.targeters, objectID)
}

// targetersOf returns a snapshot slice of charIDs currently targeting objectID.
// The slice is freshly allocated so the caller may iterate it without holding the
// lock (the loop resolves each charID to a connection and sends). nil if none.
func (ti *targetIndex) targetersOf(objectID int32) []int32 {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	s := ti.targeters[objectID]
	if len(s) == 0 {
		return nil
	}
	out := make([]int32, 0, len(s))
	for id := range s {
		out = append(out, id)
	}
	return out
}
