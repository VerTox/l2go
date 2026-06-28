package inls

import "github.com/VerTox/l2go/pkg/l2pkt"

type AuthResponse struct {
	serverID   int
	serverName string
}

func NewAuthResponse(data []byte) *AuthResponse {
	reader := l2pkt.NewReader(data)

	// Skip packet type (already consumed)
	c, _ := reader.ReadC()
	serverID := int(c)
	serverName, _ := reader.ReadS()

	return &AuthResponse{
		serverID:   serverID,
		serverName: serverName,
	}
}

func (ar *AuthResponse) GetServerID() int {
	return ar.serverID
}

func (ar *AuthResponse) GetServerName() string {
	return ar.serverName
}
