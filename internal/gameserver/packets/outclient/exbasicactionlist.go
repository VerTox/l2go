package outclient

import (
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// ExBasicActionList packet (0xFE5F) - sends available actions to client
// This is what creates the action buttons in the client UI
type ExBasicActionList struct {
	Actions []int32
}

// NewExBasicActionList creates ExBasicActionList with hardcoded Walk/Run action
func NewExBasicActionList() *ExBasicActionList {
	// Start with just the Walk/Run toggle action (Action ID 1)
	// TODO: Expand to full 189 action list later
	actions := []int32{
		1, // Walk/Run toggle - most important action
	}

	return &ExBasicActionList{
		Actions: actions,
	}
}

// BuildExBasicActionList creates the packet data
func BuildExBasicActionList(actions []int32) []byte {
	writer := l2pkt.NewWriter()

	// Extended packet format
	writer.WriteC(0xfe)                // Extended packet prefix
	writer.WriteH(0x5f)                // Extended opcode 0x5F
	writer.WriteD(int32(len(actions))) // Number of actions

	// Write each action ID as 4-byte integer
	for _, actionID := range actions {
		writer.WriteD(actionID)
	}

	return writer.Bytes()
}

// GetData returns the packet data bytes
func (p *ExBasicActionList) GetData() []byte {
	return BuildExBasicActionList(p.Actions)
}

// BuildExBasicActionListSingle creates a simple packet with just Walk/Run toggle
func BuildExBasicActionListSingle() []byte {
	return BuildExBasicActionList([]int32{1}) // Just Action ID 1 (Walk/Run)
}

func BuildDefaultExBasicActionList() []byte {
	const (
		count1 = int32(74)
		count2 = int32(99)
		count3 = int32(16)
	)
	n := count1 + count2 + count3 + 1
	DefaultActionList := make([]int32, n, 2*n)
	for i := count1; i > 0; i-- {
		DefaultActionList[i] = i
	}
	for i := count2; i > 0; i-- {
		DefaultActionList[count1+i] = 1000 + i
	}
	for i := count3; i > 0; i-- {
		DefaultActionList[count1+count2+i] = 5000 + i
	}
	return BuildExBasicActionList(DefaultActionList)
}
