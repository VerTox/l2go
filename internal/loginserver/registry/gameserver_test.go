package registry

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/loginserver/models"
)

func TestGameServerRegistry(t *testing.T) {
	ctx := context.Background()
	registry := NewGameServerRegistry()

	// Test server registration
	server := &models.GameServerInfo{
		ID:             1,
		Name:           "Test Server",
		Port:           7777,
		Status:         models.ServerStatusOnline,
		CurrentPlayers: 50,
		MaxPlayers:     1000,
		PvP:            true,
		AgeLimit:       0,
		ServerType:     1,
		ShowBrackets:   false,
	}
	// Add some test addresses
	server.AddServerAddress("127.0.0.0/8", "127.0.0.1")
	server.AddServerAddress("0.0.0.0/0", "127.0.0.1")

	// Register server
	err := registry.Register(ctx, server)
	if err != nil {
		t.Fatalf("Failed to register server: %v", err)
	}

	// Get server by ID
	retrieved, err := registry.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("Failed to get server by ID: %v", err)
	}

	if retrieved.Name != "Test Server" {
		t.Errorf("Expected server name 'Test Server', got '%s'", retrieved.Name)
	}

	// Get all servers
	servers, err := registry.GetAll(ctx)
	if err != nil {
		t.Fatalf("Failed to get all servers: %v", err)
	}

	if len(servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(servers))
	}

	// Test visible servers for normal player
	visibleServers, err := registry.GetVisibleServers(ctx, 0)
	if err != nil {
		t.Fatalf("Failed to get visible servers: %v", err)
	}

	if len(visibleServers) != 1 {
		t.Errorf("Expected 1 visible server for normal player, got %d", len(visibleServers))
	}

	// Update server status to GM-only
	err = registry.UpdateStatus(ctx, 1, models.ServerStatusGMOnly)
	if err != nil {
		t.Fatalf("Failed to update server status: %v", err)
	}

	// Test visible servers for normal player (should be 0 now)
	visibleServers, err = registry.GetVisibleServers(ctx, 0)
	if err != nil {
		t.Fatalf("Failed to get visible servers: %v", err)
	}

	if len(visibleServers) != 0 {
		t.Errorf("Expected 0 visible servers for normal player with GM-only server, got %d", len(visibleServers))
	}

	// Test visible servers for GM (should be 1)
	visibleServers, err = registry.GetVisibleServers(ctx, 1)
	if err != nil {
		t.Fatalf("Failed to get visible servers for GM: %v", err)
	}

	if len(visibleServers) != 1 {
		t.Errorf("Expected 1 visible server for GM, got %d", len(visibleServers))
	}

	// Unregister server
	err = registry.Unregister(ctx, 1)
	if err != nil {
		t.Fatalf("Failed to unregister server: %v", err)
	}

	// Verify server is gone
	servers, err = registry.GetAll(ctx)
	if err != nil {
		t.Fatalf("Failed to get all servers after unregister: %v", err)
	}

	if len(servers) != 0 {
		t.Errorf("Expected 0 servers after unregister, got %d", len(servers))
	}
}
