package outclient

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

func NewLoginOkPacket(sessionID []byte) []byte {
	buffer := new(packets.Buffer)
	buffer.WriteByte(0x03)       // Packet type: LoginOk
	buffer.Write(sessionID[:4])  // LoginOk ID 1 (4 bytes)
	buffer.Write(sessionID[4:8]) // LoginOk ID 2 (4 bytes)
	buffer.WriteUInt32(0x00)
	buffer.WriteUInt32(0x00)
	buffer.WriteUInt32(0x000003ea)
	buffer.WriteUInt32(0x00)
	buffer.WriteUInt32(0x00)
	buffer.WriteUInt32(0x00)
	// Add 16 bytes padding as per Java implementation
	for i := 0; i < 16; i++ {
		buffer.WriteByte(0x00)
	}

	return buffer.Bytes()
}
