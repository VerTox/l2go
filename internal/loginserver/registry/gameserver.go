package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/VerTox/l2go/internal/loginserver/models"
)

type GameServerRegistry interface {
	Register(ctx context.Context, info *models.GameServerInfo) error
	Unregister(ctx context.Context, id int) error
	GetAll(ctx context.Context) ([]*models.GameServerInfo, error)
	GetByID(ctx context.Context, id int) (*models.GameServerInfo, error)
	UpdateStatus(ctx context.Context, id int, status models.ServerStatus) error
	UpdatePlayerCount(ctx context.Context, id int, current, max int) error
	GetVisibleServers(ctx context.Context, accessLevel int) ([]*models.GameServerInfo, error)
}

type gameServerRegistry struct {
	mu      sync.RWMutex
	servers map[int]*models.GameServerInfo
}

func NewGameServerRegistry() GameServerRegistry {
	return &gameServerRegistry{
		servers: make(map[int]*models.GameServerInfo),
	}
}

func (r *gameServerRegistry) Register(ctx context.Context, info *models.GameServerInfo) error {
	if info == nil {
		return fmt.Errorf("gameserver info cannot be nil")
	}

	if info.ID <= 0 {
		return fmt.Errorf("invalid gameserver ID: %d", info.ID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	info.LastUpdate = time.Now()
	r.servers[info.ID] = info

	return nil
}

func (r *gameServerRegistry) Unregister(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.servers[id]; !exists {
		return fmt.Errorf("gameserver with ID %d not found", id)
	}

	delete(r.servers, id)
	return nil
}

func (r *gameServerRegistry) GetAll(ctx context.Context) ([]*models.GameServerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	servers := make([]*models.GameServerInfo, 0, len(r.servers))
	for _, server := range r.servers {
		// Create a copy to avoid race conditions
		serverCopy := *server
		servers = append(servers, &serverCopy)
	}

	return servers, nil
}

func (r *gameServerRegistry) GetByID(ctx context.Context, id int) (*models.GameServerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, exists := r.servers[id]
	if !exists {
		return nil, fmt.Errorf("gameserver with ID %d not found", id)
	}

	// Return a copy to avoid race conditions
	serverCopy := *server
	return &serverCopy, nil
}

func (r *gameServerRegistry) UpdateStatus(ctx context.Context, id int, status models.ServerStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, exists := r.servers[id]
	if !exists {
		return fmt.Errorf("gameserver with ID %d not found", id)
	}

	server.Status = status
	server.LastUpdate = time.Now()
	return nil
}

func (r *gameServerRegistry) UpdatePlayerCount(ctx context.Context, id int, current, max int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, exists := r.servers[id]
	if !exists {
		return fmt.Errorf("gameserver with ID %d not found", id)
	}

	server.CurrentPlayers = current
	server.MaxPlayers = max
	server.LastUpdate = time.Now()
	return nil
}

func (r *gameServerRegistry) GetVisibleServers(ctx context.Context, accessLevel int) ([]*models.GameServerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var visibleServers []*models.GameServerInfo
	for _, server := range r.servers {
		if server.IsVisible(accessLevel) {
			// Create a copy to avoid race conditions
			serverCopy := *server
			visibleServers = append(visibleServers, &serverCopy)
		}
	}

	return visibleServers, nil
}
