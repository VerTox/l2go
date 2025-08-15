package models

import (
	"net"
	"testing"
)

func TestGameServerInfo_AddServerAddress(t *testing.T) {
	gsi := &GameServerInfo{}

	tests := []struct {
		name       string
		subnet     string
		serverAddr string
		wantErr    bool
	}{
		{
			name:       "Valid CIDR subnet",
			subnet:     "192.168.1.0/24",
			serverAddr: "192.168.1.100",
			wantErr:    false,
		},
		{
			name:       "Single IP (auto /32)",
			subnet:     "10.0.0.1",
			serverAddr: "10.0.0.100",
			wantErr:    false,
		},
		{
			name:       "Wildcard subnet",
			subnet:     "0.0.0.0/0",
			serverAddr: "external.server.com",
			wantErr:    false,
		},
		{
			name:       "Invalid subnet",
			subnet:     "invalid-subnet",
			serverAddr: "1.2.3.4",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gsi.AddServerAddress(tt.subnet, tt.serverAddr)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddServerAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGameServerInfo_GetServerAddress(t *testing.T) {
	gsi := &GameServerInfo{}

	// Add test server addresses
	gsi.AddServerAddress("192.168.1.0/24", "192.168.1.100")
	gsi.AddServerAddress("10.0.0.0/8", "10.0.0.100")
	gsi.AddServerAddress("0.0.0.0/0", "203.0.113.50") // External/fallback

	tests := []struct {
		name     string
		clientIP string
		expected string
	}{
		{
			name:     "Local network client",
			clientIP: "192.168.1.50",
			expected: "192.168.1.100",
		},
		{
			name:     "Private network client",
			clientIP: "10.5.0.1",
			expected: "10.0.0.100",
		},
		{
			name:     "External client",
			clientIP: "8.8.8.8",
			expected: "203.0.113.50",
		},
		{
			name:     "Another external client",
			clientIP: "1.1.1.1",
			expected: "203.0.113.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientIP := net.ParseIP(tt.clientIP)
			if clientIP == nil {
				t.Fatalf("Invalid test IP: %s", tt.clientIP)
			}

			result := gsi.GetServerAddress(clientIP)
			if result != tt.expected {
				t.Errorf("GetServerAddress(%s) = %s, want %s", tt.clientIP, result, tt.expected)
			}
		})
	}
}

func TestGameServerInfo_GetExternalHost(t *testing.T) {
	gsi := &GameServerInfo{}

	// Add server addresses
	gsi.AddServerAddress("192.168.1.0/24", "192.168.1.100")
	gsi.AddServerAddress("0.0.0.0/0", "external.example.com")

	external := gsi.GetExternalHost()
	if external != "external.example.com" {
		t.Errorf("GetExternalHost() = %s, want external.example.com", external)
	}
}

func TestGameServerInfo_GetServerAddress_NoAddresses(t *testing.T) {
	gsi := &GameServerInfo{} // Empty addresses

	clientIP := net.ParseIP("192.168.1.1")
	result := gsi.GetServerAddress(clientIP)

	if result != "" {
		t.Errorf("GetServerAddress() with no addresses = %s, want empty string", result)
	}
}

func TestGameServerInfo_GetServerAddress_Fallback(t *testing.T) {
	gsi := &GameServerInfo{}

	// Only add specific subnet, no wildcard
	gsi.AddServerAddress("10.0.0.0/8", "10.0.0.100")

	// Client from different network should get fallback (first address)
	clientIP := net.ParseIP("192.168.1.1")
	result := gsi.GetServerAddress(clientIP)

	if result != "10.0.0.100" {
		t.Errorf("GetServerAddress() fallback = %s, want 10.0.0.100", result)
	}
}
