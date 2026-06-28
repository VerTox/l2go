package usecase

import (
	"context"

	"github.com/rs/zerolog/log"
)

// MockPlayerManager implements PlayerManager interface for testing/development
type MockPlayerManager struct{}

func NewMockPlayerManager() PlayerManager {
	return &MockPlayerManager{}
}

func (m *MockPlayerManager) AuthenticatePlayer(ctx context.Context, account string, sessionKey SessionKey) (bool, error) {
	log.Ctx(ctx).Warn().
		Str("account", account).
		Msg("TODO: MockPlayerManager.AuthenticatePlayer - implement actual authentication")

	// TODO: Implement actual player authentication logic
	// This should validate session keys and create player session
	return true, nil
}

func (m *MockPlayerManager) DisconnectPlayer(ctx context.Context, account string) error {
	log.Ctx(ctx).Warn().
		Str("account", account).
		Msg("TODO: MockPlayerManager.DisconnectPlayer - implement actual disconnection")

	// TODO: Implement actual player disconnection logic
	// This should find the player connection and close it
	return nil
}

func (m *MockPlayerManager) GetCharacterCount(ctx context.Context, account string) (int, int, error) {
	log.Ctx(ctx).Warn().
		Str("account", account).
		Msg("TODO: MockPlayerManager.GetCharacterCount - implement actual character counting")

	// TODO: Implement actual character counting from database
	// This should query the database for character count and deletion count
	return 0, 0, nil
}

// MockServerConfig implements ServerConfig interface for testing/development
type MockServerConfig struct {
	serverID   int
	serverName string
	maxPlayers int
	serverPort int
}

func NewMockServerConfig() ServerConfig {
	return &MockServerConfig{
		serverID:   1,
		serverName: "L2Go Test Server",
		maxPlayers: 100,
		serverPort: 7777,
	}
}

func NewServerConfigWithParams(serverID int, serverName string, maxPlayers int, serverPort int) ServerConfig {
	return &MockServerConfig{
		serverID:   serverID,
		serverName: serverName,
		maxPlayers: maxPlayers,
		serverPort: serverPort,
	}
}

func (m *MockServerConfig) GetServerID() int {
	return m.serverID
}

func (m *MockServerConfig) GetServerName() string {
	return m.serverName
}

func (m *MockServerConfig) GetMaxPlayers() int {
	return m.maxPlayers
}

func (m *MockServerConfig) GetServerPort() int {
	return m.serverPort
}
