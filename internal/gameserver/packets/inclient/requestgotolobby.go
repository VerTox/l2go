package inclient

import (
	"fmt"
)

// RequestGotoLobby packet (0xd0 0x36) - return to character selection from lobby
// Based on Java L2J: RequestGotoLobby.java  
// Format: cd (primary opcode 0xd0 + secondary opcode 0x36)
// This packet is handled as a multi-packet sub-opcode
type RequestGotoLobby struct {
	// No additional data - goto lobby packet is just the opcodes
}

// ParseRequestGotoLobby parses incoming RequestGotoLobby packet
func ParseRequestGotoLobby(data []byte) (*RequestGotoLobby, error) {
	// RequestGotoLobby packet has no payload data beyond the sub-opcode
	if len(data) < 0 {
		return nil, fmt.Errorf("RequestGotoLobby packet too short: %d bytes", len(data))
	}
	
	return &RequestGotoLobby{}, nil
}

// String returns packet information for debugging
func (p *RequestGotoLobby) String() string {
	return "RequestGotoLobby{}"
}

// Validate performs basic packet validation
func (p *RequestGotoLobby) Validate() error {
	// RequestGotoLobby packet has no data to validate
	return nil
}