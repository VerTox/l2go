package usecase

import "context"

// characterCountRepo is the slice of the character repository the player manager needs.
type characterCountRepo interface {
	GetCount(ctx context.Context, accountName string) (int, int, error) // total, inDeletion
}

// dbPlayerManager is a PlayerManager backed by the character repository. It replaces
// MockPlayerManager for the parts that have a real implementation — currently the
// character count reported to the LoginServer. (l2go-rx4)
type dbPlayerManager struct {
	characters characterCountRepo
}

// NewPlayerManager creates a PlayerManager backed by the character repository.
func NewPlayerManager(characters characterCountRepo) PlayerManager {
	return &dbPlayerManager{characters: characters}
}

// AuthenticatePlayer accepts the player: session-key validation is handled separately
// by the PlayerAuthRequest flow (LoginServer round-trip), not here.
func (m *dbPlayerManager) AuthenticatePlayer(_ context.Context, _ string, _ SessionKey) (bool, error) {
	return true, nil
}

// DisconnectPlayer is a no-op for now (forced disconnect not yet implemented).
func (m *dbPlayerManager) DisconnectPlayer(_ context.Context, _ string) error {
	return nil
}

// GetCharacterCount returns the real character/deletion counts for the account.
func (m *dbPlayerManager) GetCharacterCount(ctx context.Context, account string) (int, int, error) {
	return m.characters.GetCount(ctx, account)
}
