package outls

import (
	"fmt"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

type AuthRequest struct {
	version           int
	desiredID         int
	acceptAlternateID bool
	reserveHost       bool
	port              int
	maxPlayers        int
	hexID             []byte
	subnets           []string
	hosts             []string
}

func NewAuthRequest(desiredID int, acceptAlternateID bool, hexID []byte, port int, reserveHost bool, maxPlayers int, subnets []string, hosts []string) *AuthRequest {
	return &AuthRequest{
		version:           14, // Protocol version from Java reference
		desiredID:         desiredID,
		acceptAlternateID: acceptAlternateID,
		reserveHost:       reserveHost,
		port:              port,
		maxPlayers:        maxPlayers,
		hexID:             hexID,
		subnets:           subnets,
		hosts:             hosts,
	}
}

func (ar *AuthRequest) GetData() []byte {
	buffer := l2pkt.NewWriter()
	buffer.WriteC(0x01) // Packet type: AuthRequest
	buffer.WriteC(byte(ar.version))
	buffer.WriteC(byte(ar.desiredID))

	if ar.acceptAlternateID {
		buffer.WriteC(0x01)
	} else {
		buffer.WriteC(0x00)
	}

	if ar.reserveHost {
		buffer.WriteC(0x01)
	} else {
		buffer.WriteC(0x00)
	}

	buffer.WriteH(uint16(ar.port))
	buffer.WriteD(int32(ar.maxPlayers))
	buffer.WriteD(int32(len(ar.hexID)))
	buffer.WriteB(ar.hexID)
	buffer.WriteD(int32(len(ar.subnets)))

	fmt.Println(ar.subnets, ar.hosts)
	for i := 0; i < len(ar.subnets); i++ {
		buffer.WriteS(ar.subnets[i]) // UTF-16LE string
		fmt.Printf("writing subnet %d: %s\n", i, ar.subnets[i])
		if i < len(ar.hosts) {
			buffer.WriteS(ar.hosts[i]) // UTF-16LE string
			fmt.Printf("writing host %d: %s\n", i, ar.hosts[i])
		} else {
			buffer.WriteS("") // Empty host if not provided
			fmt.Printf("writing empty host %d\n", i)
		}
		fmt.Println("----")
	}

	return buffer.Bytes()
}
