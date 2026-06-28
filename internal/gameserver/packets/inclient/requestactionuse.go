package inclient

import (
	"fmt"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// RequestActionUse represents the client packet for action use (opcode 0x56)
// Based on Java L2J: ActionID 1 = Walk/Run toggle
// Format: ddc (actionId + ctrlPressed + shiftPressed)
type RequestActionUse struct {
	ActionID     int32 // Action ID (1 = Walk/Run toggle)
	CtrlPressed  bool  // Control key pressed
	ShiftPressed bool  // Shift key pressed
}

// ParseRequestActionUse parses a RequestActionUse packet from raw data
func ParseRequestActionUse(data []byte) (*RequestActionUse, error) {
	reader := l2pkt.NewReader(data)

	actionID, err := reader.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read action ID: %w", err)
	}

	ctrlFlag, err := reader.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read ctrl flag: %w", err)
	}

	shiftFlag, err := reader.ReadC()
	if err != nil {
		return nil, fmt.Errorf("failed to read shift flag: %w", err)
	}

	return &RequestActionUse{
		ActionID:     actionID,
		CtrlPressed:  ctrlFlag == 1,
		ShiftPressed: shiftFlag == 1,
	}, nil
}

// Validate validates the packet data
func (p *RequestActionUse) Validate() error {
	// Validate action ID range (basic check)
	if p.ActionID < 0 || p.ActionID > 1000 {
		return fmt.Errorf("invalid action ID: %d", p.ActionID)
	}
	
	return nil
}