package outclient

import (
	"encoding/binary"

	"github.com/VerTox/l2go/internal/loginserver/packets"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func NewGGAuthPacket(client *transport.Client) []byte {
	buffer := new(packets.Buffer)

	// Packet type: GGAuth (0x0B)
	buffer.WriteByte(0x0B)

	// Session ID (first 4 bytes of the 16-byte session ID)
	sessionID32 := binary.LittleEndian.Uint32(client.SessionID[:4])
	buffer.WriteUInt32(sessionID32)

	// Unknown fields (как в Java версии)
	buffer.WriteUInt32(0x00000000)
	buffer.WriteUInt32(0x00000000)
	buffer.WriteUInt32(0x00000000)
	buffer.WriteUInt32(0x00000000)

	return buffer.Bytes()
}
