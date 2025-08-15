package ings

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type AuthRequest struct {
	version         byte
	id              byte
	acceptAlternate bool
	reserveHost     bool
	port            uint16
	maxPlayers      uint32
	hexID           []byte
	subnets         []string
	hosts           []string
}

func NewAuthRequest(data []byte) *AuthRequest {
	if len(data) < 10 {
		return nil
	}

	reader := packets.NewReader(data)

	// Read version
	version := reader.ReadUInt8()

	// Read desired ID
	id := reader.ReadUInt8()

	// Read accept alternative
	acceptAlternative := reader.ReadUInt8() != 0

	// Read reserve host
	reserveHost := reader.ReadUInt8() != 0

	// Read port
	port := reader.ReadUInt16()

	// Read max players
	maxPlayers := reader.ReadUInt32()

	// Read hex ID size
	hexIDSize := reader.ReadUInt32()

	// Read hex ID
	hexID := reader.ReadBytes(int(hexIDSize))
	if len(hexID) == 0 {
		return nil // Not enough data
	}

	// Read subnets size (skip parsing actual subnets for now)
	subnetSize := reader.ReadUInt32()

	hosts := make([]string, 0)
	if subnetSize > 0 {
		for i := 0; i < int(subnetSize); i++ {
			subnet := reader.ReadString() // Assuming subnets are 16 bytes each
			hosts = append(hosts, subnet)
		}
	}

	return &AuthRequest{
		version:         version,
		id:              id,
		acceptAlternate: acceptAlternative,
		reserveHost:     reserveHost,
		port:            port,
		maxPlayers:      maxPlayers,
		hexID:           hexID,
		hosts:           hosts,
	}
}

func (ar *AuthRequest) GetID() int {
	return int(ar.id)
}

func (ar *AuthRequest) GetMaxPlayers() int {
	return int(ar.maxPlayers)
}

func (ar *AuthRequest) GetPort() int {
	return int(ar.port)
}

func (ar *AuthRequest) GetHosts() []string {
	return ar.hosts
}
