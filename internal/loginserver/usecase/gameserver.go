package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/registry"
)

type GameServerUseCase struct {
	registry registry.GameServerRegistry
}

type GameServerUseCaseParams struct {
	Registry registry.GameServerRegistry
}

func NewGameServerUseCase(params GameServerUseCaseParams) *GameServerUseCase {
	return &GameServerUseCase{
		registry: params.Registry,
	}
}

// GetServerList returns all visible game servers for the client's access level
func (uc *GameServerUseCase) GetServerList(ctx context.Context, accessLevel int) ([]*models.GameServerInfo, error) {
	servers, err := uc.registry.GetVisibleServers(ctx, accessLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to get visible servers: %w", err)
	}

	// Sort servers by ID for consistent ordering
	return servers, nil
}

// RegisterGameServer registers a new game server
func (uc *GameServerUseCase) RegisterGameServer(ctx context.Context, info *models.GameServerInfo) error {
	if info == nil {
		return fmt.Errorf("gameserver info cannot be nil")
	}

	// Set default values
	if info.Name == "" {
		info.Name = fmt.Sprintf("Server_%d", info.ID)
	}

	if info.ServerType == 0 {
		info.ServerType = 1 // Normal server by default
	}

	info.Status = models.ServerStatusOnline
	info.LastUpdate = time.Now()

	return uc.registry.Register(ctx, info)
}

// UnregisterGameServer removes a game server from registry
func (uc *GameServerUseCase) UnregisterGameServer(ctx context.Context, serverID int) error {
	return uc.registry.Unregister(ctx, serverID)
}

// UpdateServerStatus updates the status of a game server
func (uc *GameServerUseCase) UpdateServerStatus(ctx context.Context, serverID int, status models.ServerStatus) error {
	return uc.registry.UpdateStatus(ctx, serverID, status)
}

// UpdatePlayerCount updates the player count for a game server
func (uc *GameServerUseCase) UpdatePlayerCount(ctx context.Context, serverID int, current, max int) error {
	return uc.registry.UpdatePlayerCount(ctx, serverID, current, max)
}

// GetGameServer returns a specific game server by ID
func (uc *GameServerUseCase) GetGameServer(ctx context.Context, serverID int) (*models.GameServerInfo, error) {
	return uc.registry.GetByID(ctx, serverID)
}

// GetAllGameServers returns all registered game servers (for admin purposes)
func (uc *GameServerUseCase) GetAllGameServers(ctx context.Context) ([]*models.GameServerInfo, error) {
	return uc.registry.GetAll(ctx)
}
