package inclient

import (
	"fmt"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// RequestDispel is the client packet requesting removal of an active buff, sent
// when the player ctrl/right-clicks a buff icon (multi-packet 0xD0:0x4b).
// Format: D objectId, D skillId, D skillLevel (L2J RequestDispel.readImpl).
type RequestDispel struct {
	ObjectID   int32
	SkillID    int32
	SkillLevel int32
}

// ParseRequestDispel parses a RequestDispel packet (payload after the sub-opcode).
func ParseRequestDispel(data []byte) (*RequestDispel, error) {
	r := l2pkt.NewReader(data)
	objectID, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read objectId: %w", err)
	}
	skillID, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read skillId: %w", err)
	}
	skillLevel, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read skillLevel: %w", err)
	}
	return &RequestDispel{ObjectID: objectID, SkillID: skillID, SkillLevel: skillLevel}, nil
}
