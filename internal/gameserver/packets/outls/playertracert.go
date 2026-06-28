package outls

import "github.com/VerTox/l2go/pkg/l2pkt"

type PlayerTracert struct {
	account string
	pcIp    string
	hop1    string
	hop2    string
	hop3    string
	hop4    string
}

func NewPlayerTracert(account, pcIp, hop1, hop2, hop3, hop4 string) *PlayerTracert {
	return &PlayerTracert{
		account: account,
		pcIp:    pcIp,
		hop1:    hop1,
		hop2:    hop2,
		hop3:    hop3,
		hop4:    hop4,
	}
}

func (pt *PlayerTracert) GetData() []byte {
	buffer := l2pkt.NewWriter()
	buffer.WriteC(0x07)       // Packet type: PlayerTracert
	buffer.WriteS(pt.account) // UTF-16LE string
	buffer.WriteS(pt.pcIp)    // UTF-16LE string
	buffer.WriteS(pt.hop1)    // UTF-16LE string
	buffer.WriteS(pt.hop2)    // UTF-16LE string
	buffer.WriteS(pt.hop3)    // UTF-16LE string
	buffer.WriteS(pt.hop4)    // UTF-16LE string

	return buffer.Bytes()
}
