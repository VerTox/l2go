package outclient

import (
	"testing"

	"github.com/VerTox/l2go/internal/loginserver/models"
)

func TestServerList_GetServerIP(t *testing.T) {
	// Create test GameServer with multiple addresses
	server := &models.GameServerInfo{
		ID:   1,
		Name: "Test Server",
		Port: 7777,
	}

	// Add server addresses (subnet -> IP mappings)
	server.AddServerAddress("192.168.1.0/24", "192.168.1.100") // Local network
	server.AddServerAddress("10.0.0.0/8", "10.0.0.100")        // Private network
	server.AddServerAddress("0.0.0.0/0", "203.0.113.50")       // External/default

	tests := []struct {
		name       string
		clientAddr string
		expected   [4]byte
	}{
		{
			name:       "Local network client",
			clientAddr: "192.168.1.50",
			expected:   [4]byte{192, 168, 1, 100},
		},
		{
			name:       "Private network client",
			clientAddr: "10.5.0.1",
			expected:   [4]byte{10, 0, 0, 100},
		},
		{
			name:       "External client",
			clientAddr: "8.8.8.8",
			expected:   [4]byte{203, 0, 113, 50},
		},
		{
			name:       "Client with port (should extract IP)",
			clientAddr: "192.168.1.25:54321",
			expected:   [4]byte{192, 168, 1, 100},
		},
		{
			name:       "Invalid client address (fallback)",
			clientAddr: "invalid-address",
			expected:   [4]byte{203, 0, 113, 50}, // Should get external host
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			servers := []*models.GameServerInfo{server}
			serverList := NewServerList(servers, 0, tt.clientAddr, 0)

			result := serverList.getServerIP(server)
			if result != tt.expected {
				t.Errorf("getServerIP() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestServerList_GetServerIP_NoAddresses(t *testing.T) {
	// Server with no configured addresses
	server := &models.GameServerInfo{
		ID:   1,
		Name: "Empty Server",
		Port: 7777,
	}

	servers := []*models.GameServerInfo{server}
	serverList := NewServerList(servers, 0, "192.168.1.1", 0)

	result := serverList.getServerIP(server)
	expected := [4]byte{127, 0, 0, 1} // Should default to localhost

	if result != expected {
		t.Errorf("getServerIP() with no addresses = %v, want %v", result, expected)
	}
}

func TestServerList_GetServerIP_HostnameResolution(t *testing.T) {
	// Server with hostname instead of IP
	server := &models.GameServerInfo{
		ID:   1,
		Name: "Hostname Server",
		Port: 7777,
	}

	// Add localhost hostname (should resolve to 127.0.0.1)
	server.AddServerAddress("0.0.0.0/0", "localhost")

	servers := []*models.GameServerInfo{server}
	serverList := NewServerList(servers, 0, "8.8.8.8", 0)

	result := serverList.getServerIP(server)
	expected := [4]byte{127, 0, 0, 1} // localhost should resolve to 127.0.0.1

	if result != expected {
		t.Errorf("getServerIP() hostname resolution = %v, want %v", result, expected)
	}
}
