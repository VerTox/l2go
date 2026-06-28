package inclient

import (
	"fmt"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// ActionPacket represents the client Action packet (0x1F).
// Sent when the player clicks on an NPC, player, or other world object.
type ActionPacket struct {
	ObjectID int32 // Target object identifier
	OriginX  int32 // Player X when action triggered
	OriginY  int32 // Player Y when action triggered
	OriginZ  int32 // Player Z when action triggered
	ActionID byte  // 0 = simple click, 1 = shift-click
}

// ParseAction parses the Action packet payload.
func ParseAction(payload []byte) (*ActionPacket, error) {
	r := l2pkt.NewReader(payload)

	objectID, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read objectID: %w", err)
	}

	originX, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read originX: %w", err)
	}

	originY, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read originY: %w", err)
	}

	originZ, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read originZ: %w", err)
	}

	actionID, err := r.ReadC()
	if err != nil {
		return nil, fmt.Errorf("failed to read actionID: %w", err)
	}

	return &ActionPacket{
		ObjectID: objectID,
		OriginX:  originX,
		OriginY:  originY,
		OriginZ:  originZ,
		ActionID: actionID,
	}, nil
}
