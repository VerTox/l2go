package inls

import "github.com/VerTox/l2go/pkg/l2pkt"

type InitLS struct {
	rsaKey []byte
}

func NewInitLS(data []byte) *InitLS {
	reader := l2pkt.NewReader(data)

	keySize, _ := reader.ReadD()
	rsaKey := make([]byte, int(keySize))
	_ = reader.ReadB(rsaKey)

	return &InitLS{
		rsaKey: rsaKey,
	}
}

func (ils *InitLS) GetRSAKey() []byte {
	return ils.rsaKey
}
