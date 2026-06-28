package outls

import "github.com/VerTox/l2go/pkg/l2pkt"

type SessionKey struct {
	PlayOkID1  uint32
	PlayOkID2  uint32
	LoginOkID1 uint32
	LoginOkID2 uint32
}

type PlayerAuthRequest struct {
	account string
	key     SessionKey
}

func NewPlayerAuthRequest(account string, key SessionKey) *PlayerAuthRequest {
	return &PlayerAuthRequest{
		account: account,
		key:     key,
	}
}

func (par *PlayerAuthRequest) GetData() []byte {
	buffer := l2pkt.NewWriter()
	buffer.WriteC(0x05)        // Packet type: PlayerAuthRequest
	buffer.WriteS(par.account) // UTF-16LE string
	buffer.WriteD(int32(par.key.PlayOkID1))
	buffer.WriteD(int32(par.key.PlayOkID2))
	buffer.WriteD(int32(par.key.LoginOkID1))
	buffer.WriteD(int32(par.key.LoginOkID2))

	return buffer.Bytes()
}
