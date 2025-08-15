package models

import (
	"net"
	"time"
)

type ServerStatus int

const (
	ServerStatusDown ServerStatus = iota
	ServerStatusOnline
	ServerStatusGMOnly
	ServerStatusTest
)

// GameServerAddress represents a subnet -> IP mapping for client routing
type GameServerAddress struct {
	Subnet        *net.IPNet // Subnet to match against client IP
	ServerAddress string     // IP address to return to matching clients
}

type GameServerInfo struct {
	ID             int                 `json:"id"`
	Name           string              `json:"name"`
	Port           int                 `json:"port"`
	Status         ServerStatus        `json:"status"`
	CurrentPlayers int                 `json:"current_players"`
	MaxPlayers     int                 `json:"max_players"`
	PvP            bool                `json:"pvp"`
	AgeLimit       int                 `json:"age_limit"`
	ServerType     int                 `json:"server_type"` // 1: Normal, 2: Relax, 4: Test, etc.
	ShowBrackets   bool                `json:"show_brackets"`
	LastUpdate     time.Time           `json:"last_update"`
	Addresses      []GameServerAddress `json:"-"` // List of subnet->IP mappings (not serialized)
}

func (s ServerStatus) String() string {
	switch s {
	case ServerStatusDown:
		return "DOWN"
	case ServerStatusOnline:
		return "ONLINE"
	case ServerStatusGMOnly:
		return "GM_ONLY"
	case ServerStatusTest:
		return "TEST"
	default:
		return "UNKNOWN"
	}
}

func (gsi *GameServerInfo) IsOnline() bool {
	return gsi.Status == ServerStatusOnline || gsi.Status == ServerStatusGMOnly
}

func (gsi *GameServerInfo) IsVisible(accessLevel int) bool {
	if gsi.Status == ServerStatusGMOnly {
		return accessLevel > 0 // Only GMs can see GM-only servers
	}
	return gsi.Status != ServerStatusDown
}

// AddServerAddress adds a new subnet->IP mapping for this GameServer
func (gsi *GameServerInfo) AddServerAddress(subnet string, serverAddr string) error {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		// Try parsing as single IP without CIDR notation
		ip := net.ParseIP(subnet)
		if ip == nil {
			return err
		}
		// Create /32 (IPv4) or /128 (IPv6) network for single IP
		if ip.To4() != nil {
			_, ipNet, _ = net.ParseCIDR(subnet + "/32")
		} else {
			_, ipNet, _ = net.ParseCIDR(subnet + "/128")
		}
	}

	gsi.Addresses = append(gsi.Addresses, GameServerAddress{
		Subnet:        ipNet,
		ServerAddress: serverAddr,
	})

	return nil
}

// ClearServerAddresses removes all subnet->IP mappings
func (gsi *GameServerInfo) ClearServerAddresses() {
	gsi.Addresses = gsi.Addresses[:0] // Clear slice but keep capacity
}

// GetServerAddress returns the appropriate IP address for the given client IP
// This implements the same logic as Java L2J LoginServer
func (gsi *GameServerInfo) GetServerAddress(clientIP net.IP) string {
	for _, addr := range gsi.Addresses {
		if addr.Subnet.Contains(clientIP) {
			return addr.ServerAddress
		}
	}

	// Fallback: return the first address if available, or empty string
	if len(gsi.Addresses) > 0 {
		return gsi.Addresses[0].ServerAddress
	}

	return ""
}

// GetExternalHost returns the address for external clients (0.0.0.0 subnet)
// This matches Java's getExternalHost() method
func (gsi *GameServerInfo) GetExternalHost() string {
	// Look for 0.0.0.0/0 (any IP) subnet
	anyIP := net.ParseIP("0.0.0.0")
	return gsi.GetServerAddress(anyIP)
}
