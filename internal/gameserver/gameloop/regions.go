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

// regionKey returns a spatial key for the given world coordinates.
func regionKey(x, y int) string {
	return fmt.Sprintf("%d,%d", x/regionSize, y/regionSize)
}

// surroundingRegionKeys returns the 3x3 grid of region keys around a position.
func surroundingRegionKeys(x, y int) []string {
	cx := x / regionSize
	cy := y / regionSize
	keys := make([]string, 0, 9)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			keys = append(keys, fmt.Sprintf("%d,%d", cx+dx, cy+dy))
		}
	}
	return keys
}

// activateRegions activates the 3x3 region block around the player position.
func (gl *GameLoop) activateRegions(x, y int) {
	now := time.Now()
	for _, key := range surroundingRegionKeys(x, y) {
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

// deactivateRegions decrements the player count for the 3x3 region block.
func (gl *GameLoop) deactivateRegions(x, y int) {
	for _, key := range surroundingRegionKeys(x, y) {
		r, ok := gl.activeRegions[key]
		if !ok {
			continue
		}
		r.PlayerCount--
		if r.PlayerCount < 0 {
			r.PlayerCount = 0
		}
	}
}

// deactivateStaleRegions marks regions as inactive if no player was nearby for 30 seconds.
func (gl *GameLoop) deactivateStaleRegions() {
	now := time.Now()
	for key, r := range gl.activeRegions {
		if r.PlayerCount <= 0 && r.Active && now.Sub(r.LastPlayerTime) > regionDeactivateTimeout {
			r.Active = false
			delete(gl.activeRegions, key)
		}
	}
}
