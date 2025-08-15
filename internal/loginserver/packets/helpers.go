package packets

import (
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
)

func ScrambleModulus(mods []byte) []byte {
	if len(mods) != 128 {
		// Если нужно — можно паниковать или вернуть как есть; тут делаю мягкую проверку
		return mods
	}

	// Инвертируем шаги unscramble в обратном порядке:

	// d) Инверт swap: swap снова
	for i := 0; i < 4; i++ {
		mods[0x00+i], mods[0x4D+i] = mods[0x4D+i], mods[0x00+i]
	}

	// c) Инверт XOR: тот же XOR с тем же партнёром
	for i := 0; i < 0x40; i++ {
		mods[i] ^= mods[0x40+i]
	}

	// b) Инверт XOR секции 0x0D..0x10
	for i := 0; i < 4; i++ {
		mods[0x0D+i] ^= mods[0x34+i]
	}

	// a) Инверт XOR нижней половины в верхнюю
	for i := 0; i < 0x40; i++ {
		mods[0x40+i] ^= mods[i]
	}

	return mods
}

// Необязательно: прямая Go-версия твоего JS-метода для проверки раунд-трипа.
func UnscrambleModulus(mods []byte) []byte {
	if len(mods) != 128 {
		return mods
	}
	for i := 0; i < 0x40; i++ {
		mods[0x40+i] ^= mods[i]
	}
	for i := 0; i < 4; i++ {
		mods[0x0D+i] ^= mods[0x34+i]
	}
	for i := 0; i < 0x40; i++ {
		mods[i] ^= mods[0x40+i]
	}
	for i := 0; i < 4; i++ {
		mods[0x00+i], mods[0x4D+i] = mods[0x4D+i], mods[0x00+i]
	}
	return mods
}

// GetSessionIdFromSessionBytes extracts the session ID from inclient's session bytes
func GetSessionIdFromSessionBytes(sessionBytes []byte) uint32 {
	if len(sessionBytes) >= 4 {
		return binary.LittleEndian.Uint32(sessionBytes[:4])
	}
	return 0
}

func RsaDecryptNoPadding(priv *rsa.PrivateKey, cipher []byte) ([]byte, error) {
	k := priv.Size()
	if len(cipher) != k {
		return nil, errors.New(fmt.Sprintf("cipher length != key size: %d != %d", len(cipher), k))
	}

	c := new(big.Int).SetBytes(cipher)
	m := new(big.Int).Exp(c, priv.D, priv.N)
	out := m.Bytes()

	if len(out) < k {
		pad := make([]byte, k-len(out))
		out = append(pad, out...)
	}
	return out, nil
}
