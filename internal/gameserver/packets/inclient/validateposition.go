package inclient

import (
	"fmt"
	
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// ValidatePosition packet (0x59) - client position synchronization
// Based on Java L2J: ValidatePosition.java
// Format: cdddddd (opcode + 5 dwords: x, y, z, heading, data)
type ValidatePosition struct {
	X       int32 // Character X coordinate
	Y       int32 // Character Y coordinate
	Z       int32 // Character Z coordinate
	Heading int32 // Character heading (0-65535)
	Data    int32 // Vehicle ID or additional data
}

// ParseValidatePosition parses incoming ValidatePosition packet
func ParseValidatePosition(data []byte) (*ValidatePosition, error) {
	if len(data) < 20 { // 5 int32 = 20 bytes minimum
		return nil, fmt.Errorf("ValidatePosition packet too short: %d bytes", len(data))
	}
	
	reader := l2pkt.NewReader(data)
	
	packet := &ValidatePosition{}
	
	var err error
	if packet.X, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read X: %w", err)
	}
	
	if packet.Y, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read Y: %w", err)
	}
	
	if packet.Z, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read Z: %w", err)
	}
	
	if packet.Heading, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read Heading: %w", err)
	}
	
	if packet.Data, err = reader.ReadD(); err != nil {
		return nil, fmt.Errorf("failed to read Data: %w", err)
	}
	
	return packet, nil
}

// String returns packet information for debugging
func (p *ValidatePosition) String() string {
	return fmt.Sprintf("ValidatePosition{Pos:(%d,%d,%d), Heading:%d, Data:%d}", 
		p.X, p.Y, p.Z, p.Heading, p.Data)
}

// Validate performs basic packet validation
func (p *ValidatePosition) Validate() error {
	// Basic coordinate validation (L2 world bounds from Java L2J)
	const (
		MAP_MIN_X = -294912
		MAP_MAX_X = 229376
		MAP_MIN_Y = -262144
		MAP_MAX_Y = 294912
		MAP_MIN_Z = -16384
		MAP_MAX_Z = 16383
	)
	
	// Validate coordinates within L2J world bounds
	if p.X < MAP_MIN_X || p.X > MAP_MAX_X {
		return fmt.Errorf("invalid X coordinate: %d (must be between %d and %d)", 
			p.X, MAP_MIN_X, MAP_MAX_X)
	}
	
	if p.Y < MAP_MIN_Y || p.Y > MAP_MAX_Y {
		return fmt.Errorf("invalid Y coordinate: %d (must be between %d and %d)", 
			p.Y, MAP_MIN_Y, MAP_MAX_Y)
	}
	
	if p.Z < MAP_MIN_Z || p.Z > MAP_MAX_Z {
		return fmt.Errorf("invalid Z coordinate: %d (must be between %d and %d)", 
			p.Z, MAP_MIN_Z, MAP_MAX_Z)
	}
	
	// Heading validation (L2 heading is 0-65535)
	if p.Heading < 0 || p.Heading > 65535 {
		return fmt.Errorf("invalid heading: %d (must be between 0 and 65535)", p.Heading)
	}
	
	return nil
}