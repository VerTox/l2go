package inclient

import (
	"fmt"
	
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// CannotMoveAnymore packet (0x47) - movement stop notification
// Based on Java L2J: CannotMoveAnymore.java
// Sent by client when movement is stopped (obstacle, destination reached, etc.)
type CannotMoveAnymore struct {
	X       int32 // Current X position where movement stopped
	Y       int32 // Current Y position where movement stopped
	Z       int32 // Current Z position where movement stopped
	Heading int32 // Current heading direction
}

// ParseCannotMoveAnymore parses incoming CannotMoveAnymore packet
func ParseCannotMoveAnymore(data []byte) (*CannotMoveAnymore, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("CannotMoveAnymore packet too short: %d bytes, expected at least 16", len(data))
	}
	
	reader := l2pkt.NewReader(data)
	packet := &CannotMoveAnymore{}
	
	// Read position where movement stopped
	var err error
	packet.X, err = reader.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read X coordinate: %w", err)
	}
	
	packet.Y, err = reader.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read Y coordinate: %w", err)
	}
	
	packet.Z, err = reader.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read Z coordinate: %w", err)
	}
	
	packet.Heading, err = reader.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read heading: %w", err)
	}
	
	return packet, nil
}

// String returns packet information for debugging
func (p *CannotMoveAnymore) String() string {
	return fmt.Sprintf("CannotMoveAnymore{X:%d, Y:%d, Z:%d, Heading:%d}",
		p.X, p.Y, p.Z, p.Heading)
}

// Validate performs basic packet validation
func (p *CannotMoveAnymore) Validate() error {
	// Basic coordinate range validation (L2J world bounds)
	const (
		mapMinX = -294912
		mapMaxX = 229376
		mapMinY = -262144
		mapMaxY = 294912
		mapMinZ = -16384
		mapMaxZ = 16383
	)
	
	if p.X < mapMinX || p.X > mapMaxX {
		return fmt.Errorf("X coordinate %d outside valid range [%d, %d]", p.X, mapMinX, mapMaxX)
	}
	
	if p.Y < mapMinY || p.Y > mapMaxY {
		return fmt.Errorf("Y coordinate %d outside valid range [%d, %d]", p.Y, mapMinY, mapMaxY)
	}
	
	if p.Z < mapMinZ || p.Z > mapMaxZ {
		return fmt.Errorf("Z coordinate %d outside valid range [%d, %d]", p.Z, mapMinZ, mapMaxZ)
	}
	
	return nil
}

// GetPosition returns the final position where movement stopped
func (p *CannotMoveAnymore) GetPosition() (int32, int32, int32) {
	return p.X, p.Y, p.Z
}

// GetHeading returns the final heading direction
func (p *CannotMoveAnymore) GetHeading() int32 {
	return p.Heading
}