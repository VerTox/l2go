package inls

import "github.com/VerTox/l2go/pkg/l2pkt"

type LoginServerFail struct {
	reason string
}

func NewLoginServerFail(data []byte) *LoginServerFail {
	reader := l2pkt.NewReader(data)

	// Skip packet type (already consumed)
	reason, _ := reader.ReadS()

	return &LoginServerFail{
		reason: reason,
	}
}

func (lsf *LoginServerFail) GetReason() string {
	return lsf.reason
}
