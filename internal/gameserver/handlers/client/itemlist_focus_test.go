package client

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/repo"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

// fakeItemRepo implements repo.ItemRepository via interface embedding; only the
// methods actually exercised by buildItemListPacket are overridden.
type fakeItemRepo struct {
	repo.ItemRepository
	items []models.CharacterItem
	err   error
}

func (f *fakeItemRepo) GetByCharacter(ctx context.Context, charID int32) ([]models.CharacterItem, error) {
	return f.items, f.err
}

// fakeDB implements repo.DatabaseRepository via interface embedding; only Item()
// is needed for buildItemListPacket.
type fakeDB struct {
	repo.DatabaseRepository
	item repo.ItemRepository
}

func (f *fakeDB) Item() repo.ItemRepository { return f.item }

// readItemListShowWindow decodes the leading show-window short of an ItemList
// packet (opcode 0x11, then uint16 LE showWindow flag).
func readItemListShowWindow(t *testing.T, pkt []byte) uint16 {
	t.Helper()
	if len(pkt) < 3 {
		t.Fatalf("ItemList packet too short: %d bytes", len(pkt))
	}
	if pkt[0] != 0x11 {
		t.Fatalf("unexpected opcode: got 0x%x, want 0x11", pkt[0])
	}
	return binary.LittleEndian.Uint16(pkt[1:3])
}

func newHandlerWithItems(items []models.CharacterItem, err error) *Handler {
	uc := usecase.NewCharacterUseCase(&fakeDB{item: &fakeItemRepo{items: items, err: err}})
	return &Handler{characterUseCase: uc}
}

// TestBuildItemListPacket_ShowWindowFlag verifies that the leading show-window
// short reflects the requested value: RequestItemList must open the window
// (true → 1), while world entry must not (false → 0). Regression guard for
// l2go-9j1 (inventory opening unfocused / translucent).
func TestBuildItemListPacket_ShowWindowFlag(t *testing.T) {
	char := &models.Character{ID: 1}

	tests := []struct {
		name       string
		showWindow bool
		want       uint16
	}{
		{name: "request inventory opens window", showWindow: true, want: 1},
		{name: "world entry keeps window closed", showWindow: false, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandlerWithItems(nil, nil)
			pkt := h.buildItemListPacket(context.Background(), char, tt.showWindow)
			if got := readItemListShowWindow(t, pkt); got != tt.want {
				t.Fatalf("showWindow flag = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestBuildItemListPacket_ShowWindowFlag_OnError verifies the error branch also
// honors the requested show-window flag (empty inventory still opens on request).
func TestBuildItemListPacket_ShowWindowFlag_OnError(t *testing.T) {
	char := &models.Character{ID: 1}
	h := newHandlerWithItems(nil, errors.New("db down"))

	pkt := h.buildItemListPacket(context.Background(), char, true)
	if got := readItemListShowWindow(t, pkt); got != 1 {
		t.Fatalf("showWindow flag on error = %d, want 1", got)
	}
}
