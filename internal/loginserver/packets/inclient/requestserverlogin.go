package inclient

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type RequestServerLogin struct {
	SessionID []byte
	ServerID  uint8
}

func NewRequestServerLogin(request []byte) RequestServerLogin {
	var packet = packets.NewReader(request)
	var result RequestServerLogin

	result.SessionID = packet.ReadBytes(8)
	result.ServerID = packet.ReadUInt8()

	return result
}
