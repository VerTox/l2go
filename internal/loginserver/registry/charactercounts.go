package registry

import (
	"context"
	"sync"
	"time"
)

// CharacterCountInfo stores character count and cache timestamp
type CharacterCountInfo struct {
	Count     int
	Timestamp time.Time
}

// CharacterCountRegistry manages character count data for accounts across game servers
type CharacterCountRegistry interface {
	SetCharacterCount(ctx context.Context, account string, serverID int, count int) error
	GetCharacterCount(ctx context.Context, account string, serverID int) int
	GetCharacterCounts(ctx context.Context, account string) map[int]int
	ClearServer(ctx context.Context, serverID int) error
	ClearAccount(ctx context.Context, account string) error
}

// InMemoryCharacterCountRegistry provides an in-memory implementation
type InMemoryCharacterCountRegistry struct {
	// data maps account -> serverID -> CharacterCountInfo
	data     map[string]map[int]CharacterCountInfo
	mu       sync.RWMutex
	cacheTTL time.Duration
}

// NewCharacterCountRegistry creates a new character count registry
func NewCharacterCountRegistry(cacheTTL time.Duration) CharacterCountRegistry {
	return &InMemoryCharacterCountRegistry{
		data:     make(map[string]map[int]CharacterCountInfo),
		cacheTTL: cacheTTL,
	}
}

func (r *InMemoryCharacterCountRegistry) SetCharacterCount(ctx context.Context, account string, serverID int, count int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.data[account] == nil {
		r.data[account] = make(map[int]CharacterCountInfo)
	}

	r.data[account][serverID] = CharacterCountInfo{
		Count:     count,
		Timestamp: time.Now(),
	}

	return nil
}

func (r *InMemoryCharacterCountRegistry) GetCharacterCount(ctx context.Context, account string, serverID int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	serverData, exists := r.data[account]
	if !exists {
		return 0
	}

	countInfo, exists := serverData[serverID]
	if !exists {
		return 0
	}

	// Check if cached data is still valid
	if time.Since(countInfo.Timestamp) > r.cacheTTL {
		return 0 // Return 0 for expired cache entries
	}

	return countInfo.Count
}

func (r *InMemoryCharacterCountRegistry) GetCharacterCounts(ctx context.Context, account string) map[int]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[int]int)
	serverData, exists := r.data[account]
	if !exists {
		return result
	}

	now := time.Now()
	for serverID, countInfo := range serverData {
		// Only return non-expired cache entries
		if now.Sub(countInfo.Timestamp) <= r.cacheTTL {
			result[serverID] = countInfo.Count
		}
	}

	return result
}

func (r *InMemoryCharacterCountRegistry) ClearServer(ctx context.Context, serverID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove all character count data for the specified server
	for account := range r.data {
		delete(r.data[account], serverID)

		// Clean up empty account maps
		if len(r.data[account]) == 0 {
			delete(r.data, account)
		}
	}

	return nil
}

func (r *InMemoryCharacterCountRegistry) ClearAccount(ctx context.Context, account string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.data, account)
	return nil
}

// CleanupExpired removes expired cache entries
func (r *InMemoryCharacterCountRegistry) CleanupExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	for account, serverData := range r.data {
		for serverID, countInfo := range serverData {
			if now.Sub(countInfo.Timestamp) > r.cacheTTL {
				delete(serverData, serverID)
			}
		}

		// Clean up empty account maps
		if len(serverData) == 0 {
			delete(r.data, account)
		}
	}
}
