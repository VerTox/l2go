package outclient

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func NewInitPacket(client *transport.Client) []byte {
	buffer := new(packets.Buffer)

	// Packet type: Init
	buffer.WriteByte(0x00)

	// Session ID (first 4 bytes of the 16-byte session ID)
	// Write directly without double little-endian conversion
	buffer.Write(client.SessionID[:4])

	// Protocol revision (как в Java версии)
	buffer.WriteUInt32(0x0000c621)

	modulus := packets.ScrambleModulus(client.PrivateKey.PublicKey.N.Bytes())
	buffer.Write(modulus)

	// GG related unknown fields (как в Java версии)
	buffer.WriteUInt32(0x29DD954E)
	buffer.WriteUInt32(0x77C39CFC)
	buffer.WriteUInt32(0x97ADB620)
	buffer.WriteUInt32(0x07BDE0F7)

	// Blowfish key (dynamic key for this inclient)
	buffer.Write(client.BlowfishKey)

	// Null termination
	buffer.WriteByte(0x00)

	return buffer.Bytes()
}
