package inclient

import (
	"fmt"
	
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// MoveBackwardToLocation packet (0x0f) - primary movement packet from client
// Based on Java L2J: MoveBackwardToLocation.java
// Format: cdddddd (opcode + 6 dwords)
type MoveBackwardToLocation struct {
	TargetX      int32 // Destination X coordinate
	TargetY      int32 // Destination Y coordinate
	TargetZ      int32 // Destination Z coordinate
	OriginX      int32 // Current X coordinate
	OriginY      int32 // Current Y coordinate  
	OriginZ      int32 // Current Z coordinate
	MoveMovement int32 // 0 = cursor keys, 1 = mouse
}

// ParseMoveBackwardToLocation parses incoming MoveBackwardToLocation packet
func ParseMoveBackwardToLocation(data []byte) (*MoveBackwardToLocation, error) {
	if len(data) < 24 { // 6 int32 = 24 bytes minimum
		return nil, fmt.Errorf("MoveBackwardToLocation packet too short: %d bytes", len(data))
	}
	
	reader := l2pkt.NewReader(data)
	
	packet := &MoveBackwardToLocation{}
	
	var err error
	if packet.TargetX, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read TargetX: %w", err)
	}
	
	if packet.TargetY, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read TargetY: %w", err)
	}
	
	if packet.TargetZ, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read TargetZ: %w", err)
	}
	
	if packet.OriginX, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read OriginX: %w", err)
	}
	
	if packet.OriginY, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read OriginY: %w", err)
	}
	
	if packet.OriginZ, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read OriginZ: %w", err)
	}
	
	if packet.MoveMovement, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read MoveMovement: %w", err)
	}
	
	return packet, nil
}

// String returns packet information for debugging
func (p *MoveBackwardToLocation) String() string {
	return fmt.Sprintf("MoveBackwardToLocation{Target:(%d,%d,%d), Origin:(%d,%d,%d), Type:%d}", 
		p.TargetX, p.TargetY, p.TargetZ, 
		p.OriginX, p.OriginY, p.OriginZ,
		p.MoveMovement)
}

// Validate performs basic packet validation
func (p *MoveBackwardToLocation) Validate() error {
	// Basic coordinate validation (L2 world bounds from Java L2J)
	const (
		MAP_MIN_X = -294912  // L2J world boundaries
		MAP_MAX_X = 229376
		MAP_MIN_Y = -262144
		MAP_MAX_Y = 294912
		MAP_MIN_Z = -16384
		MAP_MAX_Z = 16383
	)
	
	// Validate X coordinates
	xCoords := []struct{name string; value int32}{
		{"TargetX", p.TargetX}, {"OriginX", p.OriginX},
	}
	for _, coord := range xCoords {
		if coord.value < MAP_MIN_X || coord.value > MAP_MAX_X {
			return fmt.Errorf("invalid %s coordinate: %d (must be between %d and %d)", 
				coord.name, coord.value, MAP_MIN_X, MAP_MAX_X)
		}
	}
	
	// Validate Y coordinates
	yCoords := []struct{name string; value int32}{
		{"TargetY", p.TargetY}, {"OriginY", p.OriginY},
	}
	for _, coord := range yCoords {
		if coord.value < MAP_MIN_Y || coord.value > MAP_MAX_Y {
			return fmt.Errorf("invalid %s coordinate: %d (must be between %d and %d)", 
				coord.name, coord.value, MAP_MIN_Y, MAP_MAX_Y)
		}
	}
	
	// Validate Z coordinates
	zCoords := []struct{name string; value int32}{
		{"TargetZ", p.TargetZ}, {"OriginZ", p.OriginZ},
	}
	for _, coord := range zCoords {
		if coord.value < MAP_MIN_Z || coord.value > MAP_MAX_Z {
			return fmt.Errorf("invalid %s coordinate: %d (must be between %d and %d)", 
				coord.name, coord.value, MAP_MIN_Z, MAP_MAX_Z)
		}
	}
	
	// Movement type validation
	if p.MoveMovement < 0 || p.MoveMovement > 1 {
		return fmt.Errorf("invalid MoveMovement type: %d (must be 0 or 1)", p.MoveMovement)
	}
	
	return nil
}