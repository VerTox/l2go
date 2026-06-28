package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// ChangeMoveType packet (0x28) - broadcasts character run/walk animation state change
// Based on Java L2J and TypeScript analysis: this packet tells clients to switch animations
// Format: cdd0 (opcode + character ID + isRunning flag + padding)
type ChangeMoveType struct {
	CharObjID int32 // Character object ID
	IsRunning bool  // true = running animation, false = walking animation
}

// NewChangeMoveType creates a new ChangeMoveType packet
func NewChangeMoveType(charObjID int32, isRunning bool) *ChangeMoveType {
	return &ChangeMoveType{
		CharObjID: charObjID,
		IsRunning: isRunning,
	}
}

// Build creates the binary packet data
func (p *ChangeMoveType) Build() []byte {
	writer := l2pkt.NewWriter()
	
	// Write packet opcode
	writer.WriteC(0x28)
	
	// Write character object ID
	writer.WriteD(p.CharObjID)
	
	// Write run/walk flag (1 = running, 0 = walking)
	runFlag := int32(0)
	if p.IsRunning {
		runFlag = 1
	}
	writer.WriteD(runFlag)
	
	// Write padding (based on TypeScript analysis)
	writer.WriteD(0)
	
	return writer.Bytes()
}

// GetPacketData returns packet data for l2pkt compatibility
func (p *ChangeMoveType) GetPacketData() []byte {
	return p.Build()
}

// GetData returns packet data (alternative interface)
func (p *ChangeMoveType) GetData() []byte {
	return p.Build()
}