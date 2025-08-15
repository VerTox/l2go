package ings

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"math/big"
)

type BlowFishKey struct {
	encryptedKey []byte
}

func NewBlowFishKey(data []byte) *BlowFishKey {
	if len(data) < 4 {
		// Packet too short
		return nil
	}

	// Read the size of encrypted key (4 bytes, little-endian)
	keySize := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24

	if len(data) < int(4+keySize) {
		// Packet size mismatch
		return nil
	}

	// Extract encrypted key
	encryptedKey := make([]byte, keySize)
	copy(encryptedKey, data[4:4+keySize])

	// BlowFishKey packet parsed successfully

	return &BlowFishKey{
		encryptedKey: encryptedKey,
	}
}

func (bfk *BlowFishKey) DecryptKey(privateKey *rsa.PrivateKey) ([]byte, error) {
	// Decrypt the received BlowFish key

	// Use RSA no-padding decryption (matches Java "RSA/ECB/nopadding")
	decrypted, err := rsaDecryptNoPadding(privateKey, bfk.encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt BlowFish key: %w", err)
	}

	// RSA decryption successful

	// GameServer generates variable-length Blowfish keys
	// The key is padded with zeros at the beginning during RSA encryption
	// We need to extract the actual key from the decrypted data

	// Find the first non-zero byte to locate the start of the actual key
	startIndex := -1
	for i, b := range decrypted {
		if b != 0 {
			startIndex = i
			break
		}
	}

	if startIndex >= 0 && len(decrypted)-startIndex >= 22 {
		// Try to extract 40 bytes first (GameServer default), fallback to 22
		if len(decrypted)-startIndex >= 40 {
			actualKey := decrypted[startIndex : startIndex+40]
			// Extracted 40-byte Blowfish key
			return actualKey, nil
		} else {
			// Fallback to 22 bytes for compatibility
			actualKey := decrypted[startIndex : startIndex+22]
			// Extracted 22-byte Blowfish key
			return actualKey, nil
		}
	} else if startIndex >= 0 {
		// Fallback: use all remaining non-zero bytes
		actualKey := decrypted[startIndex:]
		// Extracted variable-length Blowfish key
		return actualKey, nil
	}

	return decrypted, nil
}

// rsaDecryptNoPadding performs RSA decryption without padding (matches Java "RSA/ECB/nopadding")
func rsaDecryptNoPadding(priv *rsa.PrivateKey, cipher []byte) ([]byte, error) {
	k := priv.Size() // for 1024-bit key = 128 bytes
	if len(cipher) != k {
		return nil, errors.New(fmt.Sprintf("cipher length != key size: %d != %d", len(cipher), k))
	}
	// m = c^d mod n
	c := new(big.Int).SetBytes(cipher)
	m := new(big.Int).Exp(c, priv.D, priv.N)
	out := m.Bytes()
	// left padding with zeros to k bytes
	if len(out) < k {
		pad := make([]byte, k-len(out))
		out = append(pad, out...)
	}
	return out, nil
}
