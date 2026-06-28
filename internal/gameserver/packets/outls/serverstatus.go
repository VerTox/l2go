package outls

import "github.com/VerTox/l2go/pkg/l2pkt"

type ServerStatus struct {
	serverID     int
	serverStatus int
	port         int
	maxPlayers   int
	serverType   int
	minLevel     int
	maxLevel     int
	ageLimit     int
	showBrackets bool
	pvp          bool
	testServer   bool
	showClock    bool
}

func NewServerStatus(serverID, serverStatus, port, maxPlayers, serverType, minLevel, maxLevel, ageLimit int, showBrackets, pvp, testServer, showClock bool) *ServerStatus {
	return &ServerStatus{
		serverID:     serverID,
		serverStatus: serverStatus,
		port:         port,
		maxPlayers:   maxPlayers,
		serverType:   serverType,
		minLevel:     minLevel,
		maxLevel:     maxLevel,
		ageLimit:     ageLimit,
		showBrackets: showBrackets,
		pvp:          pvp,
		testServer:   testServer,
		showClock:    showClock,
	}
}

func (ss *ServerStatus) GetData() []byte {
	buffer := l2pkt.NewWriter()
	buffer.WriteC(0x06) // Packet type: ServerStatus
	buffer.WriteC(byte(ss.serverID))
	buffer.WriteD(int32(ss.serverStatus))
	buffer.WriteD(int32(ss.port))
	buffer.WriteD(int32(ss.maxPlayers))
	buffer.WriteD(int32(ss.serverType))
	buffer.WriteD(int32(ss.minLevel))
	buffer.WriteD(int32(ss.maxLevel))
	buffer.WriteD(int32(ss.ageLimit))

	if ss.showBrackets {
		buffer.WriteC(0x01)
	} else {
		buffer.WriteC(0x00)
	}

	if ss.pvp {
		buffer.WriteC(0x01)
	} else {
		buffer.WriteC(0x00)
	}

	if ss.testServer {
		buffer.WriteC(0x01)
	} else {
		buffer.WriteC(0x00)
	}

	if ss.showClock {
		buffer.WriteC(0x01)
	} else {
		buffer.WriteC(0x00)
	}

	return buffer.Bytes()
}
