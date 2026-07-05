package gameloop

import (
	"fmt"
	"time"
)

// ActiveRegion tracks whether a region has players nearby and should be ticked.
type ActiveRegion struct {
	Key            string
	PlayerCount    int
	LastPlayerTime time.Time
	Active         bool
}

const (
	regionSize              = 1000
	regionDeactivateTimeout = 30 * time.Second
)

// cellOf returns the grid cell (cx, cy) containing world coordinates (x, y).
func cellOf(x, y int) [2]int {
	return [2]int{x / regionSize, y / regionSize}
}

// cellKey is the activeRegions map key for a grid cell.
func cellKey(c [2]int) string {
	return fmt.Sprintf("%d,%d", c[0], c[1])
}

// surroundingKeys returns the 3x3 block of cell keys around the given center cell.
func surroundingKeys(center [2]int) []string {
	keys := make([]string, 0, 9)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			keys = append(keys, cellKey([2]int{center[0] + dx, center[1] + dy}))
		}
	}
	return keys
}

// updatePlayerRegions keeps a player's 3x3 activation block in sync with its
// position. Used for both world entry and movement. PlayerCount is a true reference
// count: a move only transitions the block when the player crosses a cell boundary —
// deactivating the old 3x3 and activating the new one — so the count no longer
// inflates on every position update and returns to 0 once players leave. (l2go-wdl)
// Loop-goroutine only (activeRegions/playerRegionCenter are loop-owned).
func (gl *GameLoop) updatePlayerRegions(charID int32, x, y int) {
	newCenter := cellOf(x, y)
	oldCenter, tracked := gl.playerRegionCenter[charID]
	if tracked && oldCenter == newCenter {
		return // still in the same cell — block activation is unchanged
	}
	if tracked {
		gl.deactivateRegionBlock(oldCenter)
	}
	gl.playerRegionCenter[charID] = newCenter
	gl.activateRegionBlock(newCenter)
}

// leavePlayerRegions deactivates the block a disconnecting player held and stops
// tracking it, so its regions can go stale and be reclaimed. (l2go-wdl)
func (gl *GameLoop) leavePlayerRegions(charID int32) {
	center, tracked := gl.playerRegionCenter[charID]
	if !tracked {
		return
	}
	gl.deactivateRegionBlock(center)
	delete(gl.playerRegionCenter, charID)
}

// activateRegionBlock increments the reference count on the 3x3 block around center.
func (gl *GameLoop) activateRegionBlock(center [2]int) {
	now := time.Now()
	for _, key := range surroundingKeys(center) {
		r, ok := gl.activeRegions[key]
		if !ok {
			r = &ActiveRegion{Key: key}
			gl.activeRegions[key] = r
		}
		r.PlayerCount++
		r.LastPlayerTime = now
		r.Active = true
	}
}

// deactivateRegionBlock decrements the reference count on the 3x3 block around
// center and stamps LastPlayerTime, so the 30s vacancy timeout starts from the
// moment the player left (not when it first arrived).
func (gl *GameLoop) deactivateRegionBlock(center [2]int) {
	now := time.Now()
	for _, key := range surroundingKeys(center) {
		r, ok := gl.activeRegions[key]
		if !ok {
			continue
		}
		r.PlayerCount--
		if r.PlayerCount < 0 {
			r.PlayerCount = 0
		}
		r.LastPlayerTime = now
	}
}

// deactivateStaleRegions removes regions that have had no players for the timeout.
// With correct ref-counting a vacated region's PlayerCount returns to 0, so this now
// actually reclaims entries instead of leaking them. (l2go-wdl)
func (gl *GameLoop) deactivateStaleRegions() {
	now := time.Now()
	for key, r := range gl.activeRegions {
		if r.PlayerCount <= 0 && now.Sub(r.LastPlayerTime) > regionDeactivateTimeout {
			delete(gl.activeRegions, key)
		}
	}
}
