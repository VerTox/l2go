package outclient

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type PlayOk struct {
	playOkID1 uint32
	playOkID2 uint32
}

func NewPlayOk(playOkID1, playOkID2 uint32) *PlayOk {
	return &PlayOk{
		playOkID1: playOkID1,
		playOkID2: playOkID2,
	}
}

func (p *PlayOk) GetData() []byte {
	buffer := new(packets.Buffer)
	buffer.WriteByte(0x07)          // Opcode: PlayOk
	buffer.WriteUInt32(p.playOkID1) // PlayOk ID 1 (4 bytes)
	buffer.WriteUInt32(p.playOkID2) // PlayOk ID 2 (4 bytes)

	return buffer.Bytes()
}

// Legacy function for backward compatibility
func NewPlayOkPacket(playOkID1, playOkID2 uint32) []byte {
	playOk := NewPlayOk(playOkID1, playOkID2)
	return playOk.GetData()
}
