package inclient

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type RequestPlay struct {
	ServerID  uint8
	SessionID []byte
}

func NewRequestPlay(request []byte) RequestPlay {
	var packet = packets.NewReader(request)
	var result RequestPlay

	result.SessionID = packet.ReadBytes(8)
	result.ServerID = packet.ReadUInt8()

	return result
}
