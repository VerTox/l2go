package outgs

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

// RequestCharacters packet sent from LoginServer to GameServer to request character count for an account
// Opcode: 0x05
type RequestCharacters struct {
	Account string
}

func NewRequestCharacters(account string) *RequestCharacters {
	return &RequestCharacters{
		Account: account,
	}
}

func (p *RequestCharacters) GetData() []byte {
	buffer := new(packets.Buffer)

	// Opcode 0x05
	buffer.WriteByte(0x05)

	// Account name as UTF-16 string
	buffer.WriteString(p.Account)

	return buffer.Bytes()
}
