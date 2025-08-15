package outclient

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

// PlayFail reasons
const (
	PLAY_REASON_ACCESS_FAILED     = 0x04
	PLAY_REASON_SERVER_OVERLOADED = 0x0F
	PLAY_REASON_TEMP_BAN          = 0x10
)

type PlayFail struct {
	reason uint32
}

func NewPlayFail(reason uint32) *PlayFail {
	return &PlayFail{
		reason: reason,
	}
}

func (p *PlayFail) GetData() []byte {
	buffer := new(packets.Buffer)
	buffer.WriteByte(0x06) // Packet type: PlayFail
	buffer.WriteUInt32(p.reason)

	return buffer.Bytes()
}

// Legacy function for backward compatibility
func NewPlayFailPacket(reason uint32) []byte {
	playFail := NewPlayFail(reason)
	return playFail.GetData()
}
