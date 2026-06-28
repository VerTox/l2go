package inclient

import (
	"fmt"
)

// Logout packet (0x00) - complete game exit
// Based on Java L2J: Logout.java
// Format: c (opcode only, no additional data)
type Logout struct {
	// No additional data - logout packet is just the opcode
}

// ParseLogout parses incoming Logout packet
func ParseLogout(data []byte) (*Logout, error) {
	// Logout packet has no payload data, just validate minimum length
	if len(data) < 0 {
		return nil, fmt.Errorf("Logout packet too short: %d bytes", len(data))
	}
	
	return &Logout{}, nil
}

// String returns packet information for debugging
func (p *Logout) String() string {
	return "Logout{}"
}

// Validate performs basic packet validation
func (p *Logout) Validate() error {
	// Logout packet has no data to validate
	return nil
}