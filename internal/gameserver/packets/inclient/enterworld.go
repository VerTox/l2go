package inclient

// EnterWorld (opcode 0x11) - client enters the world after character selection.
type EnterWorld struct{}

func NewEnterWorld(_ []byte) *EnterWorld { return &EnterWorld{} }