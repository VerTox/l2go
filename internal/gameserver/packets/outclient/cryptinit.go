package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// DefaultXORKey returns the default initial XOR key used by the protocol.
func DefaultXORKey() []byte { return []byte{0x94, 0x35, 0x00, 0x00, 0xa1, 0x6c, 0x54, 0x87} }

// NewCryptInitPacket builds the CryptInit packet sent by server to client.
func NewCryptInitPacket(key []byte) []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x00) // opcode: CryptInit
	b.WriteC(0x01) // unknown/flag (legacy-compatible)
	b.WriteB(key)
	return b.Bytes()
}
