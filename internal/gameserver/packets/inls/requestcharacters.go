package inls

import "github.com/VerTox/l2go/pkg/l2pkt"

type RequestCharacters struct {
	account string
}

func NewRequestCharacters(data []byte) *RequestCharacters {
	reader := l2pkt.NewReader(data)

	// Skip packet type (already consumed)
	account, _ := reader.ReadS()

	return &RequestCharacters{
		account: account,
	}
}

func (rc *RequestCharacters) GetAccount() string {
	return rc.account
}
