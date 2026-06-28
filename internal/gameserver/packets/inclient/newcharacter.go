package inclient

// NewCharacter (opcode 0x13) – client requests character creation templates.
type NewCharacter struct{}

func NewNewCharacter(_ []byte) *NewCharacter { return &NewCharacter{} }

