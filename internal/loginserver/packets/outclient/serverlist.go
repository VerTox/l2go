package outclient

import (
	"net"

	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type ServerList struct {
	servers         []*models.GameServerInfo
	lastServer      int
	clientAddr      string
	accessLevel     int
	characterCounts map[int]int // serverID -> character count
}

func NewServerList(servers []*models.GameServerInfo, lastServer int, clientAddr string, accessLevel int) *ServerList {
	return &ServerList{
		servers:         servers,
		lastServer:      lastServer,
		clientAddr:      clientAddr,
		accessLevel:     accessLevel,
		characterCounts: make(map[int]int),
	}
}

func NewServerListWithCharCounts(servers []*models.GameServerInfo, lastServer int, clientAddr string, accessLevel int, characterCounts map[int]int) *ServerList {
	return &ServerList{
		servers:         servers,
		lastServer:      lastServer,
		clientAddr:      clientAddr,
		accessLevel:     accessLevel,
		characterCounts: characterCounts,
	}
}

func (p *ServerList) GetData() []byte {
	buffer := new(packets.Buffer)

	// Packet opcode
	buffer.WriteByte(0x04)

	// Server count
	buffer.WriteUInt8(uint8(len(p.servers)))

	// Last server ID
	buffer.WriteUInt8(uint8(p.lastServer))

	// Write each server info
	for _, server := range p.servers {
		// Server ID
		buffer.WriteUInt8(uint8(server.ID))

		// Server IP (4 bytes)
		ip := p.getServerIP(server)
		buffer.WriteByte(ip[0])
		buffer.WriteByte(ip[1])
		buffer.WriteByte(ip[2])
		buffer.WriteByte(ip[3])

		// Server port
		buffer.WriteUInt32(uint32(server.Port))

		// Age limit (0, 15, 18)
		buffer.WriteUInt8(uint8(server.AgeLimit))

		// PvP flag
		if server.PvP {
			buffer.WriteByte(0x01)
		} else {
			buffer.WriteByte(0x00)
		}

		// Current players
		buffer.WriteUInt16(uint16(server.CurrentPlayers))

		// Max players
		buffer.WriteUInt16(uint16(server.MaxPlayers))

		// Server status (0 = down, 1 = online)
		if server.IsOnline() && server.IsVisible(p.accessLevel) {
			buffer.WriteByte(0x01)
		} else {
			buffer.WriteByte(0x00)
		}

		// Server type flags
		buffer.WriteUInt32(uint32(server.ServerType))

		// Show brackets flag
		if server.ShowBrackets {
			buffer.WriteByte(0x01)
		} else {
			buffer.WriteByte(0x00)
		}
	}

	// Unknown field (always 0x00)
	buffer.WriteUInt16(0x00)

	// Characters on servers section
	p.writeCharacterCounts(buffer)

	return buffer.Bytes()
}

// writeCharacterCounts writes character count data for servers
func (p *ServerList) writeCharacterCounts(buffer *packets.Buffer) {
	// Count how many servers have character data
	serverCount := 0
	for _, server := range p.servers {
		if _, exists := p.characterCounts[server.ID]; exists {
			serverCount++
		}
	}

	// Write character count section
	buffer.WriteUInt8(uint8(serverCount))

	// Write character counts for each server that has data
	for _, server := range p.servers {
		if charCount, exists := p.characterCounts[server.ID]; exists {
			buffer.WriteUInt8(uint8(server.ID))
			buffer.WriteUInt8(uint8(charCount))
		}
	}
}

// getServerIP resolves server IP based on client address using subnet matching
func (p *ServerList) getServerIP(server *models.GameServerInfo) [4]byte {
	// Parse client IP address
	clientIP := net.ParseIP(p.clientAddr)
	if clientIP == nil {
		// If can't parse client IP, try to extract from host:port format
		host, _, err := net.SplitHostPort(p.clientAddr)
		if err == nil {
			clientIP = net.ParseIP(host)
		}
	}

	var selectedIP string

	// Use new subnet-based IP selection if client IP is available
	if clientIP != nil {
		selectedIP = server.GetServerAddress(clientIP)
	}

	// Fallback to external host if no match found
	if selectedIP == "" {
		selectedIP = server.GetExternalHost()
	}

	// Final fallback to localhost
	if selectedIP == "" {
		return [4]byte{127, 0, 0, 1}
	}

	// Parse the selected IP
	ip := net.ParseIP(selectedIP)
	if ip == nil {
		// Try resolving hostname to IP
		ips, err := net.LookupIP(selectedIP)
		if err == nil && len(ips) > 0 {
			ip = ips[0]
		}
	}

	if ip == nil {
		// Default to localhost if IP parsing/resolution fails
		return [4]byte{127, 0, 0, 1}
	}

	// Convert to 4-byte IPv4
	if ipv4 := ip.To4(); ipv4 != nil {
		return [4]byte{ipv4[0], ipv4[1], ipv4[2], ipv4[3]}
	}

	// Default to localhost for IPv6 or invalid IPs
	return [4]byte{127, 0, 0, 1}
}

// Legacy function for backward compatibility
func NewServerListPacket(servers []*models.GameServerInfo, remoteAddr string) []byte {
	serverList := NewServerList(servers, 0, remoteAddr, 0)
	return serverList.GetData()
}
