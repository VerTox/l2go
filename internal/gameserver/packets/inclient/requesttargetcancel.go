package inclient

import (
	"fmt"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// RequestTargetCancel represents the client packet (0x48).
// Sent when the player presses Escape or clicks to deselect a target.
type RequestTargetCancel struct {
	Unselect int16 // 0 = cancel target / abort cast, non-zero = explicit deselect
}

// ParseRequestTargetCancel parses the RequestTargetCancel packet payload.
func ParseRequestTargetCancel(payload []byte) (*RequestTargetCancel, error) {
	r := l2pkt.NewReader(payload)

	v, err := r.ReadH()
	if err != nil {
		return nil, fmt.Errorf("failed to read unselect: %w", err)
	}

	return &RequestTargetCancel{
		Unselect: int16(v),
	}, nil
}
