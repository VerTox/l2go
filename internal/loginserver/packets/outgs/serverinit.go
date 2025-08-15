package outgs

import (
	"crypto/rsa"

	"github.com/VerTox/l2go/internal/loginserver/packets"
)

func NewInitPacket(sessionID []byte, blowfishKey []byte, privateKey *rsa.PrivateKey) []byte {
	buffer := new(packets.Buffer)

	// Packet type: Init
	buffer.WriteByte(0x00)

	// Session ID (first 4 bytes of the 16-byte session ID)
	// Write directly without double little-endian conversion
	buffer.Write(sessionID[:4])

	// Protocol revision (как в Java версии)
	buffer.WriteUInt32(0x0000c621)

	modulus := packets.ScrambleModulus(privateKey.PublicKey.N.Bytes())
	buffer.Write(modulus)

	// GG related unknown fields (как в Java версии)
	buffer.WriteUInt32(0x29DD954E)
	buffer.WriteUInt32(0x77C39CFC)
	buffer.WriteUInt32(0x97ADB620)
	buffer.WriteUInt32(0x07BDE0F7)

	// Blowfish key (dynamic key for this inclient)
	buffer.Write(blowfishKey)

	// Null termination
	buffer.WriteByte(0x00)

	return buffer.Bytes()
}
