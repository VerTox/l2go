package gameloop

import (
	"math"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// interpolatePosition returns the position along the straight line from start to
// dest after `elapsed` of the total travel time, and whether the move completed.
// Linear interpolation (no geodata), matching movementUseCase.
func interpolatePosition(start, dest models.Position, elapsed, total time.Duration) (models.Position, bool) {
	if total <= 0 || elapsed >= total {
		return dest, true
	}
	if elapsed <= 0 {
		return start, false
	}
	progress := float64(elapsed) / float64(total)
	return models.Position{
		X: start.X + int(float64(dest.X-start.X)*progress),
		Y: start.Y + int(float64(dest.Y-start.Y)*progress),
		Z: start.Z + int(float64(dest.Z-start.Z)*progress),
	}, false
}

// distanceBetween is the 2D distance between two positions.
func distanceBetween(a, b models.Position) float64 {
	dx := float64(b.X - a.X)
	dy := float64(b.Y - a.Y)
	return math.Sqrt(dx*dx + dy*dy)
}
