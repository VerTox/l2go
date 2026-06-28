package outls

import (
	"crypto/rsa"
	"math/big"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

type BlowFishKey struct {
	BlowfishKey []byte
	PublicKey   *rsa.PublicKey
}

func NewBlowFishKey(blowfishKey []byte, publicKey *rsa.PublicKey) *BlowFishKey {
	return &BlowFishKey{
		BlowfishKey: blowfishKey,
		PublicKey:   publicKey,
	}
}

func (p BlowFishKey) Write(w *l2pkt.Writer) {
	w.WriteC(0x00)
	m := new(big.Int).SetBytes(p.BlowfishKey)
	c := new(big.Int).Exp(m, big.NewInt(int64(p.PublicKey.E)), p.PublicKey.N)
	encrypted := c.Bytes()
	actualKeySize := (p.PublicKey.N.BitLen() + 7) / 8
	if len(encrypted) < actualKeySize {
		// Pad with leading zeros
		padded := make([]byte, actualKeySize)
		copy(padded[actualKeySize-len(encrypted):], encrypted)
		encrypted = padded
	}
	w.WriteD(int32(len(encrypted)))
	w.WriteB(encrypted)
}
