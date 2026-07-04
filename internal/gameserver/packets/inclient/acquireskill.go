package inclient

import (
	"fmt"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// RequestAcquireSkillPacket is both RequestAcquireSkillInfo (0x73) and
// RequestAcquireSkill (0x7c): D id, D level, D skillType. (l2go-hv9)
type RequestAcquireSkillPacket struct {
	SkillID   int32
	Level     int32
	SkillType int32
}

// ParseRequestAcquireSkill parses the common (id, level, type) acquire-skill layout.
func ParseRequestAcquireSkill(payload []byte) (*RequestAcquireSkillPacket, error) {
	r := l2pkt.NewReader(payload)
	id, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read skillID: %w", err)
	}
	level, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read level: %w", err)
	}
	skillType, err := r.ReadD()
	if err != nil {
		return nil, fmt.Errorf("read skillType: %w", err)
	}
	return &RequestAcquireSkillPacket{SkillID: id, Level: level, SkillType: skillType}, nil
}

// RequestBypassPacket is RequestBypassToServer (0x23): S command. (l2go-hv9)
type RequestBypassPacket struct {
	Command string
}

// ParseRequestBypass parses the bypass command string.
func ParseRequestBypass(payload []byte) (*RequestBypassPacket, error) {
	r := l2pkt.NewReader(payload)
	cmd, err := r.ReadS()
	if err != nil {
		return nil, fmt.Errorf("read bypass command: %w", err)
	}
	return &RequestBypassPacket{Command: cmd}, nil
}
