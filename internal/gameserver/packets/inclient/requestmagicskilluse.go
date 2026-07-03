package inclient

import (
	"fmt"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// RequestMagicSkillUse is the client packet requesting a skill cast (opcode 0x39).
// Format: ddc — magicId + ctrlPressed + shiftPressed. Matches L2J HF
// RequestMagicSkillUse.readImpl.
type RequestMagicSkillUse struct {
	MagicID      int32
	CtrlPressed  bool // Ctrl held = force attack
	ShiftPressed bool // Shift held = cast without approaching
}

// ParseRequestMagicSkillUse parses a RequestMagicSkillUse packet.
func ParseRequestMagicSkillUse(data []byte) (*RequestMagicSkillUse, error) {
	reader := l2pkt.NewReader(data)

	magicID, err := reader.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read magic ID: %w", err)
	}
	ctrlFlag, err := reader.ReadD()
	if err != nil {
		return nil, fmt.Errorf("failed to read ctrl flag: %w", err)
	}
	shiftFlag, err := reader.ReadC()
	if err != nil {
		return nil, fmt.Errorf("failed to read shift flag: %w", err)
	}

	return &RequestMagicSkillUse{
		MagicID:      magicID,
		CtrlPressed:  ctrlFlag != 0,
		ShiftPressed: shiftFlag != 0,
	}, nil
}
