package inclient

import (
	"fmt"
)

// RequestRestart packet (0x57) - return to character selection from in-game
// Based on Java L2J: RequestRestart.java
// Format: c (opcode only, no additional data)
type RequestRestart struct {
	// No additional data - restart packet is just the opcode
}

// ParseRequestRestart parses incoming RequestRestart packet
func ParseRequestRestart(data []byte) (*RequestRestart, error) {
	// RequestRestart packet has no payload data, just validate minimum length
	if len(data) < 0 {
		return nil, fmt.Errorf("RequestRestart packet too short: %d bytes", len(data))
	}
	
	return &RequestRestart{}, nil
}

// String returns packet information for debugging
func (p *RequestRestart) String() string {
	return "RequestRestart{}"
}

// Validate performs basic packet validation
func (p *RequestRestart) Validate() error {
	// RequestRestart packet has no data to validate
	return nil
}