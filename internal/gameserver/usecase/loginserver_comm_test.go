package usecase

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

type fakeCharCountRepo struct {
	total, del int
	gotAccount string
}

func (f *fakeCharCountRepo) GetCount(_ context.Context, account string) (int, int, error) {
	f.gotAccount = account
	return f.total, f.del, nil
}

// TestPlayerManagerGetCharacterCount verifies the real player manager delegates the
// count to the character repository. (l2go-rx4)
func TestPlayerManagerGetCharacterCount(t *testing.T) {
	repo := &fakeCharCountRepo{total: 3, del: 1}
	pm := NewPlayerManager(repo)

	total, del, err := pm.GetCharacterCount(context.Background(), "vertox")
	if err != nil {
		t.Fatalf("GetCharacterCount: %v", err)
	}
	if total != 3 || del != 1 {
		t.Errorf("counts = %d,%d, want 3,1", total, del)
	}
	if repo.gotAccount != "vertox" {
		t.Errorf("account passed to repo = %q, want vertox", repo.gotAccount)
	}
}

type fakePlayerManager struct{ total, del int }

func (f *fakePlayerManager) AuthenticatePlayer(context.Context, string, SessionKey) (bool, error) {
	return true, nil
}
func (f *fakePlayerManager) DisconnectPlayer(context.Context, string) error { return nil }
func (f *fakePlayerManager) GetCharacterCount(context.Context, string) (int, int, error) {
	return f.total, f.del, nil
}

// TestHandleRequestCharactersSendsReply verifies the handler forwards the account's
// character counts to the ReplyCharacters sender. (l2go-rx4)
func TestHandleRequestCharactersSendsReply(t *testing.T) {
	var gotAcc string
	var gotCount, gotDel int
	called := false

	uc := &LoginServerCommUseCaseImpl{
		playerManager: &fakePlayerManager{total: 5, del: 2},
		onSendReplyCharacters: func(account string, charCount, charsInDel int) error {
			called = true
			gotAcc, gotCount, gotDel = account, charCount, charsInDel
			return nil
		},
	}

	w := l2pkt.NewWriter()
	w.WriteS("vertox")
	pkt := inls.NewRequestCharacters(w.Bytes())

	if err := uc.HandleRequestCharacters(context.Background(), pkt); err != nil {
		t.Fatalf("HandleRequestCharacters: %v", err)
	}
	if !called {
		t.Fatal("ReplyCharacters sender was not called")
	}
	if gotAcc != "vertox" || gotCount != 5 || gotDel != 2 {
		t.Errorf("reply = (%q, %d, %d), want (vertox, 5, 2)", gotAcc, gotCount, gotDel)
	}
}
