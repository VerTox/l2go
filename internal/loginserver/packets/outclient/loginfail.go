package outclient

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

// Login fail reasons
const (
	REASON_SYSTEM_ERROR       = 0x01
	REASON_USER_OR_PASS_WRONG = 0x02
	REASON_ACCESS_FAILED      = 0x04
	REASON_ACCOUNT_IN_USE     = 0x07
	REASON_SERVER_OVERLOADED  = 0x0F
)

func NewLoginFailPacket(reason uint32) []byte {
	buffer := new(packets.Buffer)
	buffer.WriteByte(0x01) // Packet type: LoginFail
	buffer.WriteUInt32(reason)

	return buffer.Bytes()
}
