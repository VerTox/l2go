package inls

import "github.com/VerTox/l2go/pkg/l2pkt"

type PlayerAuthResponse struct {
	account string
	authed  bool
}

func NewPlayerAuthResponse(data []byte) *PlayerAuthResponse {
	reader := l2pkt.NewReader(data)

	// Skip packet type (already consumed)
	account, _ := reader.ReadS()
	c, _ := reader.ReadC()
	authed := c != 0

	return &PlayerAuthResponse{
		account: account,
		authed:  authed,
	}
}

func (par *PlayerAuthResponse) GetAccount() string {
	return par.account
}

func (par *PlayerAuthResponse) IsAuthed() bool {
	return par.authed
}
