package inclient

import (
	"fmt"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// Restart point types (L2J RequestRestartPoint._requestedPointType).
const (
	RestartPointTown = 0 // resurrect in the nearest village
)

// RequestRestartPoint represents the client packet (0x7d): the player chose a
// resurrection point after death.
type RequestRestartPoint struct {
	RequestedPointType int32
}

// ParseRequestRestartPoint parses the RequestRestartPoint payload (a single int32).
func ParseRequestRestartPoint(payload []byte) (*RequestRestartPoint, error) {
	r := l2pkt.NewReader(payload)

	v, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read requested point type: %w", err)
	}

	return &RequestRestartPoint{RequestedPointType: v}, nil
}
