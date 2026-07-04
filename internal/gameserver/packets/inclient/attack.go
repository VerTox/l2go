package inclient

import (
	"fmt"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// AttackPacket is the client Attack packet (0x01) — Ctrl force-attack. Layout
// cddddc: objectId, originX, originY, originZ, attackId (0 simple / 1 shift).
type AttackPacket struct {
	ObjectID int32
	OriginX  int32
	OriginY  int32
	OriginZ  int32
	AttackID byte
}

// ParseAttack parses the Attack packet payload.
func ParseAttack(payload []byte) (*AttackPacket, error) {
	r := l2pkt.NewReader(payload)
	objectID, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read objectID: %w", err)
	}
	ox, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read originX: %w", err)
	}
	oy, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read originY: %w", err)
	}
	oz, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read originZ: %w", err)
	}
	aid, err := r.ReadC()
	if err != nil {
		return nil, fmt.Errorf("read attackID: %w", err)
	}
	return &AttackPacket{ObjectID: objectID, OriginX: ox, OriginY: oy, OriginZ: oz, AttackID: aid}, nil
}
