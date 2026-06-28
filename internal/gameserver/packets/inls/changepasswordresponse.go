package inls

import "github.com/VerTox/l2go/pkg/l2pkt"

type ChangePasswordResponse struct {
	account    string
	hasChanged bool
}

func NewChangePasswordResponse(data []byte) *ChangePasswordResponse {
	reader := l2pkt.NewReader(data)

	// Skip packet type (already consumed)
	account, _ := reader.ReadS()
	c, _ := reader.ReadC()
	hasChanged := c != 0

	return &ChangePasswordResponse{
		account:    account,
		hasChanged: hasChanged,
	}
}

func (cpr *ChangePasswordResponse) GetAccount() string {
	return cpr.account
}

func (cpr *ChangePasswordResponse) HasChanged() bool {
	return cpr.hasChanged
}
