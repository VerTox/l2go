package outgs

import "github.com/VerTox/l2go/internal/loginserver/packets"

type PlayerAuthResponse struct {
	account string
	success bool
}

func NewPlayerAuthResponse(account string, success bool) *PlayerAuthResponse {
	return &PlayerAuthResponse{
		account: account,
		success: success,
	}
}

func (par *PlayerAuthResponse) GetData() []byte {
	buffer := new(packets.Buffer)
	buffer.WriteByte(0x03) // Packet type: PlayerAuthResponse
	buffer.WriteString(par.account)

	if par.success {
		buffer.WriteByte(0x01) // Success
	} else {
		buffer.WriteByte(0x00) // Failure
	}

	return buffer.Bytes()
}
