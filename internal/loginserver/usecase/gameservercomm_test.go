package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/packets/ings"
	"github.com/VerTox/l2go/internal/loginserver/registry"
)

func TestGameServerCommUseCase(t *testing.T) {
	ctx := context.Background()

	// Setup dependencies
	gameServerRegistry := registry.NewGameServerRegistry()
	sessionUseCase := NewSessionUseCase(1 * time.Minute)

	uc := NewGameServerCommUseCase(GameServerCommParams{
		GameServerRegistry: gameServerRegistry,
		SessionUseCase:     sessionUseCase,
	})

	// Test HandleAuthRequest
	t.Run("HandleAuthRequest", func(t *testing.T) {
		// Create mock AuthRequest data
		data := make([]byte, 20)
		data[0] = 1 // version
		data[1] = 1 // server ID
		data[2] = 0 // accept alternative
		data[3] = 0 // reserve host
		// port (little-endian)
		data[4] = 0x39 // 7777 & 0xFF
		data[5] = 0x1E // (7777 >> 8) & 0xFF
		// max players (little-endian)
		data[6] = 0xE8 // 1000 & 0xFF
		data[7] = 0x03 // (1000 >> 8) & 0xFF
		data[8] = 0x00
		data[9] = 0x00
		// hex ID size
		data[10] = 0x01
		data[11] = 0x00
		data[12] = 0x00
		data[13] = 0x00
		// subnets size
		data[14] = 0x00
		data[15] = 0x00
		data[16] = 0x00
		data[17] = 0x00

		authReq := ings.NewAuthRequest(data)
		if authReq == nil {
			t.Fatal("Failed to create AuthRequest")
		}

		result, err := uc.HandleAuthRequest(ctx, authReq)
		if err != nil {
			t.Fatalf("HandleAuthRequest failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected auth request to succeed, got: %s", result.Reason)
		}

		if result.ServerID != 1 {
			t.Errorf("Expected server ID 1, got %d", result.ServerID)
		}

		if result.ServerName != "Bartz" {
			t.Errorf("Expected server name 'Bartz', got '%s'", result.ServerName)
		}

		// Verify server was registered
		servers, err := gameServerRegistry.GetAll(ctx)
		if err != nil {
			t.Fatalf("Failed to get servers: %v", err)
		}

		if len(servers) != 1 {
			t.Errorf("Expected 1 server registered, got %d", len(servers))
		}

		server := servers[0]
		if server.ID != 1 {
			t.Errorf("Expected registered server ID 1, got %d", server.ID)
		}

		if server.Name != "Bartz" {
			t.Errorf("Expected registered server name 'Bartz', got '%s'", server.Name)
		}
	})

	// Test SessionKey validation through usecase
	t.Run("SessionKeyValidation", func(t *testing.T) {
		// First create a session key
		sessionKey, err := sessionUseCase.CreateSessionKey(ctx, "testuser", 1, 0x11111111, 0x22222222)
		if err != nil {
			t.Fatalf("Failed to create session key: %v", err)
		}

		// Test valid session key
		testSessionKey := models.SessionKey{
			LoginKey1: sessionKey.LoginKey1,
			LoginKey2: sessionKey.LoginKey2,
			PlayKey1:  sessionKey.PlayKey1,
			PlayKey2:  sessionKey.PlayKey2,
		}

		valid, err := sessionUseCase.ValidateSessionKey(ctx, "testuser", testSessionKey)
		if err != nil {
			t.Fatalf("Session validation failed: %v", err)
		}

		if !valid {
			t.Error("Expected session key to be valid")
		}
	})

	// Test HandlePlayerLogout
	t.Run("HandlePlayerLogout", func(t *testing.T) {
		// Create a session key first
		_, err := sessionUseCase.CreateSessionKey(ctx, "logoutuser", 1, 0x33333333, 0x44444444)
		if err != nil {
			t.Fatalf("Failed to create session key: %v", err)
		}

		// Test logout
		err = uc.HandlePlayerLogout(ctx, "logoutuser", 1)
		if err != nil {
			t.Errorf("HandlePlayerLogout failed: %v", err)
		}

		// Verify session key was removed
		_, err = sessionUseCase.GetSessionKey(ctx, "logoutuser")
		if err == nil {
			t.Error("Expected session key to be removed after logout")
		}
	})

	// Test server name generation
	t.Run("GenerateServerName", func(t *testing.T) {
		testCases := map[int]string{
			1:  "Bartz",
			2:  "Sieghardt",
			15: "Chronos",
			99: "Server_99", // Unknown ID
		}

		for serverID, expectedName := range testCases {
			name := uc.generateServerName(serverID)
			if name != expectedName {
				t.Errorf("Server ID %d: expected name '%s', got '%s'", serverID, expectedName, name)
			}
		}
	})
}
