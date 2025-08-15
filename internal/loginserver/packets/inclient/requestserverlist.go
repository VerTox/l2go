package inclient

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type RequestServerList struct {
	SessionID []byte
}

func NewRequestServerList(request []byte) RequestServerList {
	var packet = packets.NewReader(request)
	var result RequestServerList

	result.SessionID = packet.ReadBytes(8)

	return result
}
