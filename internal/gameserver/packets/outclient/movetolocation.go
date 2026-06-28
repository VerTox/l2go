package outclient

import (
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// MoveToLocation packet (0x2f) - broadcasts character movement to clients  
// Based on Java L2J: MoveToLocation.java
// Format: cddddddd (opcode + character ID + 6 coordinates)
type MoveToLocation struct {
	CharObjID int32 // Character object ID
	XDst      int32 // Destination X coordinate
	YDst      int32 // Destination Y coordinate
	ZDst      int32 // Destination Z coordinate
	X         int32 // Current X coordinate
	Y         int32 // Current Y coordinate
	Z         int32 // Current Z coordinate
}

// NewMoveToLocation creates a new MoveToLocation packet
func NewMoveToLocation(charObjID int32, dstX, dstY, dstZ, currentX, currentY, currentZ int32) *MoveToLocation {
	return &MoveToLocation{
		CharObjID: charObjID,
		XDst:      dstX,
		YDst:      dstY,
		ZDst:      dstZ,
		X:         currentX,
		Y:         currentY,
		Z:         currentZ,
	}
}

// Build creates the binary packet data
func (p *MoveToLocation) Build() []byte {
	writer := l2pkt.NewWriter()
	
	// Write packet opcode
	writer.WriteC(0x2f)
	
	// Write character object ID
	writer.WriteD(p.CharObjID)
	
	// Write destination coordinates
	writer.WriteD(p.XDst)
	writer.WriteD(p.YDst)
	writer.WriteD(p.ZDst)
	
	// Write current coordinates
	writer.WriteD(p.X)
	writer.WriteD(p.Y)
	writer.WriteD(p.Z)
	
	return writer.Bytes()
}

// GetPacketData returns packet data for l2pkt compatibility
func (p *MoveToLocation) GetPacketData() []byte {
	return p.Build()
}

// GetData returns packet data (alternative interface)
func (p *MoveToLocation) GetData() []byte {
	return p.Build()
}