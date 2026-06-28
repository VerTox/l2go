package outclient

import (
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// ValidatePosition packet (server → client) - forces client position correction
// Used for anti-cheat and position synchronization
// This is a custom implementation for server-side position validation
type ValidatePositionServer struct {
	CharObjID int32 // Character object ID
	X         int32 // Correct X coordinate
	Y         int32 // Correct Y coordinate  
	Z         int32 // Correct Z coordinate
	Heading   int32 // Correct heading (0-65535)
}

// NewValidatePositionServer creates a new ValidatePosition server packet
func NewValidatePositionServer(charObjID int32, x, y, z, heading int32) *ValidatePositionServer {
	return &ValidatePositionServer{
		CharObjID: charObjID,
		X:         x,
		Y:         y,
		Z:         z,
		Heading:   heading,
	}
}

// Build creates the binary packet data
func (p *ValidatePositionServer) Build() []byte {
	writer := l2pkt.NewWriter()
	
	// Note: This opcode might need adjustment based on L2J protocol
	// Using a custom opcode for server-side position validation
	writer.WriteC(0x61) // Custom opcode for server position validation
	
	// Write character object ID
	writer.WriteD(p.CharObjID)
	
	// Write coordinates
	writer.WriteD(p.X)
	writer.WriteD(p.Y)
	writer.WriteD(p.Z)
	
	// Write heading
	writer.WriteD(p.Heading)
	
	return writer.Bytes()
}

// GetPacketData returns packet data for l2pkt compatibility
func (p *ValidatePositionServer) GetPacketData() []byte {
	return p.Build()
}

// GetData returns packet data (alternative interface)
func (p *ValidatePositionServer) GetData() []byte {
	return p.Build()
}