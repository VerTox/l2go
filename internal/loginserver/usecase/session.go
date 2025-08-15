package usecase

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/models"
)

type SessionUseCase struct {
	sessionKeys      map[string]*models.SessionKey // account -> session key
	sessionKeysMutex sync.RWMutex
	maxSessionAge    time.Duration
}

func NewSessionUseCase(maxSessionAge time.Duration) *SessionUseCase {
	if maxSessionAge == 0 {
		maxSessionAge = 5 * time.Minute // Default session expiration
	}

	return &SessionUseCase{
		sessionKeys:   make(map[string]*models.SessionKey),
		maxSessionAge: maxSessionAge,
	}
}

// CreateSessionKey creates a new session key for the specified account and server
func (uc *SessionUseCase) CreateSessionKey(ctx context.Context, account string, serverID int, loginOkID1, loginOkID2 uint32) (*models.SessionKey, error) {
	playKey1, err := uc.generateRandomUint32()
	if err != nil {
		return nil, fmt.Errorf("failed to generate playKey1: %w", err)
	}

	playKey2, err := uc.generateRandomUint32()
	if err != nil {
		return nil, fmt.Errorf("failed to generate playKey2: %w", err)
	}

	sessionKey := &models.SessionKey{
		LoginKey1: loginOkID1,
		LoginKey2: loginOkID2,
		PlayKey1:  playKey1,
		PlayKey2:  playKey2,
		Account:   account,
		ServerID:  serverID,
		CreatedAt: time.Now(),
	}

	uc.sessionKeysMutex.Lock()
	defer uc.sessionKeysMutex.Unlock()

	// Remove any existing session key for this account
	if existingKey, exists := uc.sessionKeys[account]; exists {
		log.Ctx(ctx).Debug().
			Str("account", account).
			Time("old_created_at", existingKey.CreatedAt).
			Msg("Replacing existing session key")
	}

	uc.sessionKeys[account] = sessionKey

	log.Ctx(ctx).Info().
		Str("account", account).
		Int("server_id", serverID).
		Uint32("login_key1", loginOkID1).
		Uint32("login_key2", loginOkID2).
		Uint32("play_key1", playKey1).
		Uint32("play_key2", playKey2).
		Msg("Created session key")

	return sessionKey, nil
}

// ValidateSessionKey validates a session key for the specified account
func (uc *SessionUseCase) ValidateSessionKey(ctx context.Context, account string, requestedKey models.SessionKey) (bool, error) {
	uc.sessionKeysMutex.RLock()
	defer uc.sessionKeysMutex.RUnlock()

	storedKey, exists := uc.sessionKeys[account]
	if !exists {
		log.Ctx(ctx).Warn().
			Str("account", account).
			Msg("Session key validation failed - no stored session key found")
		return false, nil
	}

	// Check if session key is expired
	if storedKey.IsExpired(uc.maxSessionAge) {
		log.Ctx(ctx).Warn().
			Str("account", account).
			Time("created_at", storedKey.CreatedAt).
			Dur("max_age", uc.maxSessionAge).
			Msg("Session key validation failed - session expired")

		// Clean up expired session key
		delete(uc.sessionKeys, account)
		return false, nil
	}

	// Validate keys match
	if !storedKey.Equals(requestedKey) {
		log.Ctx(ctx).Warn().
			Str("account", account).
			Uint32("expected_login1", storedKey.LoginKey1).
			Uint32("expected_login2", storedKey.LoginKey2).
			Uint32("expected_play1", storedKey.PlayKey1).
			Uint32("expected_play2", storedKey.PlayKey2).
			Uint32("received_login1", requestedKey.LoginKey1).
			Uint32("received_login2", requestedKey.LoginKey2).
			Uint32("received_play1", requestedKey.PlayKey1).
			Uint32("received_play2", requestedKey.PlayKey2).
			Msg("Session key validation failed - key mismatch")
		return false, nil
	}

	log.Ctx(ctx).Info().
		Str("account", account).
		Msg("Session key validated successfully")

	return true, nil
}

// RemoveSessionKey removes the session key for the specified account
func (uc *SessionUseCase) RemoveSessionKey(ctx context.Context, account string) error {
	uc.sessionKeysMutex.Lock()
	defer uc.sessionKeysMutex.Unlock()

	if _, exists := uc.sessionKeys[account]; exists {
		delete(uc.sessionKeys, account)
		log.Ctx(ctx).Info().Str("account", account).Msg("Session key removed")
		return nil
	}

	return fmt.Errorf("session key not found for account: %s", account)
}

// GetSessionKey returns the session key for the specified account
func (uc *SessionUseCase) GetSessionKey(ctx context.Context, account string) (*models.SessionKey, error) {
	uc.sessionKeysMutex.RLock()
	defer uc.sessionKeysMutex.RUnlock()

	sessionKey, exists := uc.sessionKeys[account]
	if !exists {
		return nil, fmt.Errorf("session key not found for account: %s", account)
	}

	// Check if expired
	if sessionKey.IsExpired(uc.maxSessionAge) {
		return nil, fmt.Errorf("session key expired for account: %s", account)
	}

	return sessionKey, nil
}

// CleanupExpiredSessions removes all expired session keys
func (uc *SessionUseCase) CleanupExpiredSessions(ctx context.Context) int {
	uc.sessionKeysMutex.Lock()
	defer uc.sessionKeysMutex.Unlock()

	var expiredCount int
	for account, sessionKey := range uc.sessionKeys {
		if sessionKey.IsExpired(uc.maxSessionAge) {
			delete(uc.sessionKeys, account)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		log.Ctx(ctx).Info().
			Int("expired_count", expiredCount).
			Msg("Cleaned up expired session keys")
	}

	return expiredCount
}

// GetSessionCount returns the current number of active sessions
func (uc *SessionUseCase) GetSessionCount() int {
	uc.sessionKeysMutex.RLock()
	defer uc.sessionKeysMutex.RUnlock()
	return len(uc.sessionKeys)
}

// generateRandomUint32 generates a cryptographically secure random uint32
func (uc *SessionUseCase) generateRandomUint32() (uint32, error) {
	var bytes [4]byte
	_, err := rand.Read(bytes[:])
	if err != nil {
		return 0, err
	}
	return uint32(bytes[0]) | (uint32(bytes[1]) << 8) | (uint32(bytes[2]) << 16) | (uint32(bytes[3]) << 24), nil
}
