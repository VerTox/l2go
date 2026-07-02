package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/packets/inclient"
	"github.com/VerTox/l2go/internal/loginserver/repo"
)

// mockAccountRepo is a minimal in-memory AccountRepository for tests.
type mockAccountRepo struct {
	updateLastServerCalls int
	lastAccountID         int
	lastServerID          int
	updateLastServerErr   error
}

func (m *mockAccountRepo) GetByUsername(ctx context.Context, username string) (*models.Account, error) {
	return nil, nil
}

func (m *mockAccountRepo) Create(ctx context.Context, account *models.Account) error { return nil }

func (m *mockAccountRepo) Update(ctx context.Context, account *models.Account) error { return nil }

func (m *mockAccountRepo) UpdateLastServer(ctx context.Context, accountID, serverID int) error {
	m.updateLastServerCalls++
	m.lastAccountID = accountID
	m.lastServerID = serverID
	return m.updateLastServerErr
}

func (m *mockAccountRepo) Delete(ctx context.Context, id int) error { return nil }

func TestHandleRequestServerLogin_PersistsLastServer(t *testing.T) {
	ctx := context.Background()
	mock := &mockAccountRepo{}
	uc := NewClientUseCase(Params{
		Repo:           repo.Repository{Account: mock},
		SessionUseCase: NewSessionUseCase(1 * time.Minute),
	})

	account := &models.Account{ID: 42, Username: "hero", LastServer: 0}
	req := &inclient.RequestServerLogin{ServerID: 3}

	result, err := uc.HandleRequestServerLogin(ctx, req, account, 0x1111, 0x2222)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got reason %q", result.Reason)
	}

	if mock.updateLastServerCalls != 1 {
		t.Errorf("UpdateLastServer called %d times, want 1", mock.updateLastServerCalls)
	}
	if mock.lastAccountID != 42 {
		t.Errorf("persisted account id = %d, want 42", mock.lastAccountID)
	}
	if mock.lastServerID != 3 {
		t.Errorf("persisted server id = %d, want 3", mock.lastServerID)
	}
	if account.LastServer != 3 {
		t.Errorf("in-memory account.LastServer = %d, want 3", account.LastServer)
	}
}

func TestHandleRequestServerLogin_NoAccount(t *testing.T) {
	ctx := context.Background()
	mock := &mockAccountRepo{}
	uc := NewClientUseCase(Params{
		Repo:           repo.Repository{Account: mock},
		SessionUseCase: NewSessionUseCase(1 * time.Minute),
	})

	req := &inclient.RequestServerLogin{ServerID: 3}
	result, err := uc.HandleRequestServerLogin(ctx, req, nil, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for nil account")
	}
	if mock.updateLastServerCalls != 0 {
		t.Errorf("UpdateLastServer should not be called for nil account, got %d calls", mock.updateLastServerCalls)
	}
}
