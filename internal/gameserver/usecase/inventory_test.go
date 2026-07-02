package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// --- test doubles for UseItem (distinct names from potionhandler_test.go) ---

// invItemRepo implements just the repo.ItemRepository methods UseItem touches.
type invItemRepo struct {
	repo.ItemRepository // embedded: unimplemented methods panic if called
	items               map[int32]*models.CharacterItem
	deleted             map[int32]bool
}

func (r *invItemRepo) GetByObjectID(_ context.Context, objectID int32) (*models.CharacterItem, error) {
	return r.items[objectID], nil
}

func (r *invItemRepo) Update(_ context.Context, _ *models.CharacterItem) error { return nil }

func (r *invItemRepo) Delete(_ context.Context, objectID int32) error {
	if r.deleted == nil {
		r.deleted = map[int32]bool{}
	}
	r.deleted[objectID] = true
	return nil
}

type invRepo struct {
	repo.DatabaseRepository
	item *invItemRepo
}

func (r *invRepo) Item() repo.ItemRepository { return r.item }

// stubConsumeHandler consumes one unit and reports consumed=true, like a potion.
type stubConsumeHandler struct{ consume bool }

func (h *stubConsumeHandler) UseItem(_ context.Context, use ItemUseContext) (bool, error) {
	if h.consume {
		use.Item.Count--
	}
	return h.consume, nil
}

// newUseItemTest builds an InventoryUseCase wired with a fresh reuse registry, an
// injectable clock, and a template resolver backed by an in-memory map.
func newUseItemTest(items map[int32]*models.CharacterItem, tmpls map[int32]*registry.ItemTemplate) (*InventoryUseCase, *time.Time) {
	clock := time.Unix(1_000_000, 0)
	uc := &InventoryUseCase{
		repo:         &invRepo{item: &invItemRepo{items: items}},
		itemHandlers: NewItemHandlerRegistry(),
		reuse:        registry.NewItemReuseRegistry(),
		now:          func() time.Time { return clock },
		templateOf:   func(id int32) *registry.ItemTemplate { return tmpls[id] },
	}
	return uc, &clock
}

func TestUseItem_QuestItemRefused(t *testing.T) {
	item := &models.CharacterItem{ObjectID: 500, ItemID: 7000, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{ID: 7000, Type2: registry.ItemType2Quest}
	uc, _ := newUseItemTest(map[int32]*models.CharacterItem{500: item}, map[int32]*registry.ItemTemplate{7000: tmpl})

	res, err := uc.UseItem(context.Background(), 7, 500, PlayerCondition{})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if res.Success {
		t.Error("quest item use should not succeed")
	}
	if len(res.Messages) != 1 || res.Messages[0].ID != sysMsgCannotUseQuestItems {
		t.Errorf("messages = %+v, want CANNOT_USE_QUEST_ITEMS", res.Messages)
	}
	if item.Count != 1 {
		t.Errorf("quest item consumed (count=%d), want untouched", item.Count)
	}
}

func TestUseItem_DeadRefused(t *testing.T) {
	item := &models.CharacterItem{ObjectID: 501, ItemID: 1539, OwnerID: 7, Count: 3}
	tmpl := &registry.ItemTemplate{ID: 1539, Handler: "ItemSkills"}
	uc, _ := newUseItemTest(map[int32]*models.CharacterItem{501: item}, map[int32]*registry.ItemTemplate{1539: tmpl})
	uc.itemHandlers.Register("ItemSkills", &stubConsumeHandler{consume: true})

	res, err := uc.UseItem(context.Background(), 7, 501, PlayerCondition{IsDead: true})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if res.Success {
		t.Error("dead player use should not succeed")
	}
	if len(res.Messages) != 1 || res.Messages[0].ID != sysMsgS1CannotBeUsed || res.Messages[0].ItemName != 1539 {
		t.Errorf("messages = %+v, want S1_CANNOT_BE_USED with item name 1539", res.Messages)
	}
	if item.Count != 3 {
		t.Errorf("item consumed while dead (count=%d), want 3", item.Count)
	}
}

func TestUseItem_ArmsReuseThenBlocksThenExpires(t *testing.T) {
	item := &models.CharacterItem{ObjectID: 502, ItemID: 1540, OwnerID: 7, Count: 10}
	tmpl := &registry.ItemTemplate{ID: 1540, Handler: "ItemSkills", ReuseDelay: 15000} // 15s, no group
	uc, clock := newUseItemTest(map[int32]*models.CharacterItem{502: item}, map[int32]*registry.ItemTemplate{1540: tmpl})
	uc.itemHandlers.Register("ItemSkills", &stubConsumeHandler{consume: true})

	// First use succeeds and arms the cooldown.
	res, err := uc.UseItem(context.Background(), 7, 502, PlayerCondition{})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if !res.Success || item.Count != 9 {
		t.Fatalf("first use: success=%v count=%d, want success and count 9", res.Success, item.Count)
	}
	// No shared reuse group -> no ExUseSharedGroupItem.
	if res.ReuseSync != nil {
		t.Errorf("ReuseSync = %+v, want nil (no group)", res.ReuseSync)
	}

	// Second use within cooldown is refused with a reuse-remaining message.
	*clock = clock.Add(5 * time.Second)
	res, err = uc.UseItem(context.Background(), 7, 502, PlayerCondition{})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if res.Success {
		t.Error("use during cooldown should be refused")
	}
	if item.Count != 9 {
		t.Errorf("item consumed during cooldown (count=%d), want 9", item.Count)
	}
	if len(res.Messages) != 1 || res.Messages[0].ID != sysMsgSecReuseS1 || res.Messages[0].ItemName != 1540 {
		t.Fatalf("messages = %+v, want SECONDS_REMAINING_FOR_REUSE with item 1540", res.Messages)
	}
	// 15s total, 5s elapsed -> 10s remaining.
	if got := res.Messages[0].Ints; len(got) != 1 || got[0] != 10 {
		t.Errorf("remaining seconds = %v, want [10]", got)
	}

	// After the cooldown elapses, use succeeds again.
	*clock = clock.Add(11 * time.Second) // total elapsed 16s > 15s
	res, err = uc.UseItem(context.Background(), 7, 502, PlayerCondition{})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if !res.Success || item.Count != 8 {
		t.Errorf("post-cooldown use: success=%v count=%d, want success and count 8", res.Success, item.Count)
	}
}

func TestUseItem_SharedGroupBlocksSiblingAndSyncs(t *testing.T) {
	// Two distinct items sharing reuse group 42.
	itemA := &models.CharacterItem{ObjectID: 600, ItemID: 8000, OwnerID: 7, Count: 5}
	itemB := &models.CharacterItem{ObjectID: 601, ItemID: 8001, OwnerID: 7, Count: 5}
	tmplA := &registry.ItemTemplate{ID: 8000, Handler: "ItemSkills", ReuseDelay: 20000, SharedReuseGroup: 42}
	tmplB := &registry.ItemTemplate{ID: 8001, Handler: "ItemSkills", ReuseDelay: 20000, SharedReuseGroup: 42}
	uc, clock := newUseItemTest(
		map[int32]*models.CharacterItem{600: itemA, 601: itemB},
		map[int32]*registry.ItemTemplate{8000: tmplA, 8001: tmplB},
	)
	uc.itemHandlers.Register("ItemSkills", &stubConsumeHandler{consume: true})

	// Use item A -> arms group 42, emits ExUseSharedGroupItem for the group.
	res, err := uc.UseItem(context.Background(), 7, 600, PlayerCondition{})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if !res.Success {
		t.Fatal("first use should succeed")
	}
	if res.ReuseSync == nil || res.ReuseSync.GroupID != 42 || res.ReuseSync.ItemID != 8000 {
		t.Fatalf("ReuseSync = %+v, want group 42 item 8000", res.ReuseSync)
	}
	if res.ReuseSync.Remaining != 20*time.Second || res.ReuseSync.Total != 20*time.Second {
		t.Errorf("ReuseSync timings = %v/%v, want 20s/20s", res.ReuseSync.Remaining, res.ReuseSync.Total)
	}

	// Item B (sibling in group 42) is blocked by the shared cooldown.
	*clock = clock.Add(6 * time.Second)
	res, err = uc.UseItem(context.Background(), 7, 601, PlayerCondition{})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if res.Success {
		t.Error("sibling in shared group should be blocked")
	}
	if itemB.Count != 5 {
		t.Errorf("sibling consumed (count=%d), want 5", itemB.Count)
	}
	// Blocked-during-cooldown still syncs the group icon.
	if res.ReuseSync == nil || res.ReuseSync.GroupID != 42 || res.ReuseSync.ItemID != 8001 {
		t.Fatalf("blocked ReuseSync = %+v, want group 42 item 8001", res.ReuseSync)
	}
	if res.ReuseSync.Remaining != 14*time.Second {
		t.Errorf("blocked remaining = %v, want 14s", res.ReuseSync.Remaining)
	}
	if len(res.Messages) != 1 || res.Messages[0].ID != sysMsgSecReuseS1 || res.Messages[0].Ints[0] != 14 {
		t.Errorf("blocked message = %+v, want reuse-seconds 14", res.Messages)
	}
}

func TestUseItem_NoReuseDelayNoArm(t *testing.T) {
	item := &models.CharacterItem{ObjectID: 700, ItemID: 9000, OwnerID: 7, Count: 4}
	tmpl := &registry.ItemTemplate{ID: 9000, Handler: "ItemSkills"} // ReuseDelay 0
	uc, clock := newUseItemTest(map[int32]*models.CharacterItem{700: item}, map[int32]*registry.ItemTemplate{9000: tmpl})
	uc.itemHandlers.Register("ItemSkills", &stubConsumeHandler{consume: true})

	res, _ := uc.UseItem(context.Background(), 7, 700, PlayerCondition{})
	if !res.Success || res.ReuseSync != nil {
		t.Fatalf("first use: success=%v reuseSync=%+v", res.Success, res.ReuseSync)
	}
	// Immediately usable again (no cooldown armed).
	*clock = clock.Add(time.Millisecond)
	res, _ = uc.UseItem(context.Background(), 7, 700, PlayerCondition{})
	if !res.Success || item.Count != 2 {
		t.Errorf("second use: success=%v count=%d, want success and count 2", res.Success, item.Count)
	}
}

func TestUseItem_ReuseMessageFormats(t *testing.T) {
	cases := []struct {
		name      string
		remaining time.Duration
		wantID    int32
		wantInts  []int32
	}{
		{"seconds", 42 * time.Second, sysMsgSecReuseS1, []int32{42}},
		{"minutes", 2*time.Minute + 5*time.Second, sysMsgMinSecReuseS1, []int32{2, 5}},
		{"hours", 1*time.Hour + 3*time.Minute + 7*time.Second, sysMsgHourMinSecReuseS1, []int32{1, 3, 7}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := reuseSysMsg(1234, tc.remaining)
			if m.ID != tc.wantID || m.ItemName != 1234 {
				t.Fatalf("msg = %+v, want id %d item 1234", m, tc.wantID)
			}
			if len(m.Ints) != len(tc.wantInts) {
				t.Fatalf("ints = %v, want %v", m.Ints, tc.wantInts)
			}
			for i := range tc.wantInts {
				if m.Ints[i] != tc.wantInts[i] {
					t.Errorf("ints[%d] = %d, want %d", i, m.Ints[i], tc.wantInts[i])
				}
			}
		})
	}
}
