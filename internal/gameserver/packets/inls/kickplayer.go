package inls

import "github.com/VerTox/l2go/pkg/l2pkt"

type KickPlayer struct {
	account string
}

func NewKickPlayer(data []byte) *KickPlayer {
	reader := l2pkt.NewReader(data)

	// Skip packet type (already consumed)
	account, _ := reader.ReadS()

	return &KickPlayer{
		account: account,
	}
}

func (kp *KickPlayer) GetAccount() string {
	return kp.account
}
