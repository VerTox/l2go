package outgs

import "github.com/VerTox/l2go/internal/loginserver/packets"

type AuthResponse struct {
	serverID   int
	serverName string
}

func NewAuthResponse(serverID int, serverName string) *AuthResponse {
	return &AuthResponse{
		serverID:   serverID,
		serverName: serverName,
	}
}

func (ar *AuthResponse) GetData() []byte {
	buffer := new(packets.Buffer)
	buffer.WriteByte(0x02) // Packet type: AuthResponse
	buffer.WriteByte(byte(ar.serverID))
	buffer.WriteString(ar.serverName) // UTF-16LE string

	return buffer.Bytes()
}
