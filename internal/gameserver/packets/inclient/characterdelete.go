package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// CharacterDelete (opcode 0x0d) - client requests to delete a character.
type CharacterDelete struct {
	CharacterSlot int32
}

func (p *CharacterDelete) Read(r *l2pkt.Reader) bool {
	slot, err := r.ReadD()
	if err != nil {
		return false
	}
	p.CharacterSlot = slot

	return true
}

func NewCharacterDelete(_ []byte) *CharacterDelete { return &CharacterDelete{} }