package outls

import "github.com/VerTox/l2go/pkg/l2pkt"

type PlayerLogout struct {
	account string
}

func NewPlayerLogout(account string) *PlayerLogout {
	return &PlayerLogout{
		account: account,
	}
}

func (plo *PlayerLogout) GetData() []byte {
	buffer := l2pkt.NewWriter()
	buffer.WriteC(0x03)        // Packet type: PlayerLogout
	buffer.WriteS(plo.account) // UTF-16LE string

	return buffer.Bytes()
}
