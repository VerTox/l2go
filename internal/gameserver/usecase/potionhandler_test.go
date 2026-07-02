package usecase

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// --- test doubles ---

// fakeSkillSource returns canned effects keyed by "id-level".
type fakeSkillSource struct {
	effects map[[2]int]registry.SkillEffect
}

func (f *fakeSkillSource) Lookup(id, level int) (registry.SkillEffect, bool) {
	e, ok := f.effects[[2]int{id, level}]
	return e, ok
}

// recordingRestorer captures RestoreStats calls.
type recordingRestorer struct {
	calls      int
	charID     int32
	hp, mp, cp int32
}

func (r *recordingRestorer) RestoreStats(charID, hp, mp, cp int32) {
	r.calls++
	r.charID = charID
	r.hp, r.mp, r.cp = hp, mp, cp
}

// fakeItemRepo implements repo.ItemRepository just enough for consumption.
type fakeItemRepo struct {
	repo.ItemRepository // embedded: unimplemented methods panic if called
	updated             *models.CharacterItem
	deleted             int32
}

func (f *fakeItemRepo) Update(_ context.Context, item *models.CharacterItem) error {
	f.updated = item
	return nil
}

func (f *fakeItemRepo) Delete(_ context.Context, objectID int32) error {
	f.deleted = objectID
	return nil
}

// fakeRepo implements repo.DatabaseRepository, exposing only Item().
type fakeRepo struct {
	repo.DatabaseRepository // embedded: unimplemented methods panic if called
	item                    *fakeItemRepo
}

func (f *fakeRepo) Item() repo.ItemRepository { return f.item }

func newPotionTest(effects map[[2]int]registry.SkillEffect) (*PotionHandler, *recordingRestorer, *fakeItemRepo) {
	restorer := &recordingRestorer{}
	itemRepo := &fakeItemRepo{}
	h := NewPotionHandler(&fakeSkillSource{effects: effects}, restorer)
	return h, restorer, itemRepo
}

func TestPotionHandler_RestoresHPAndConsumes(t *testing.T) {
	h, restorer, itemRepo := newPotionTest(map[[2]int]registry.SkillEffect{
		{2037, 1}: {Kind: registry.EffectHP, Amount: 50},
	})
	repository := &fakeRepo{item: itemRepo}

	item := &models.CharacterItem{ObjectID: 500, ItemID: 1539, OwnerID: 7, Count: 3}
	tmpl := &registry.ItemTemplate{
		ID:         1539,
		Name:       "Greater Healing Potion",
		Handler:    "ItemSkills",
		ItemSkills: []registry.ItemSkill{{ID: 2037, Level: 1}},
	}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: item, Template: tmpl, Repo: repository,
	})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true")
	}
	// Restore routed with HP=50, no MP/CP.
	if restorer.calls != 1 || restorer.charID != 7 || restorer.hp != 50 || restorer.mp != 0 || restorer.cp != 0 {
		t.Errorf("restorer = %+v, want 1 call charID=7 hp=50", restorer)
	}
	// One potion consumed (3 -> 2), item updated not deleted.
	if item.Count != 2 {
		t.Errorf("item.Count = %d, want 2", item.Count)
	}
	if itemRepo.updated == nil || itemRepo.updated.Count != 2 {
		t.Errorf("expected Update with count 2, got %+v", itemRepo.updated)
	}
	if itemRepo.deleted != 0 {
		t.Errorf("item should not be deleted, deleted=%d", itemRepo.deleted)
	}
}

func TestPotionHandler_LastItemDeleted(t *testing.T) {
	h, _, itemRepo := newPotionTest(map[[2]int]registry.SkillEffect{
		{10001, 1}: {Kind: registry.EffectMP, Amount: 100},
	})
	repository := &fakeRepo{item: itemRepo}

	item := &models.CharacterItem{ObjectID: 501, ItemID: 728, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{
		ID: 728, Handler: "ManaPotion",
		ItemSkills: []registry.ItemSkill{{ID: 10001, Level: 1}},
	}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: repository})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true")
	}
	if item.Count != 0 {
		t.Errorf("item.Count = %d, want 0", item.Count)
	}
	if itemRepo.deleted != 501 {
		t.Errorf("expected item 501 deleted, got %d", itemRepo.deleted)
	}
}

func TestPotionHandler_NoSkillsIsNoOp(t *testing.T) {
	h, restorer, itemRepo := newPotionTest(nil)
	repository := &fakeRepo{item: itemRepo}

	item := &models.CharacterItem{ObjectID: 502, ItemID: 1, OwnerID: 7, Count: 5}
	tmpl := &registry.ItemTemplate{ID: 1, Handler: "ItemSkills"} // no ItemSkills

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: repository})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false when no restore effect resolved")
	}
	if restorer.calls != 0 {
		t.Errorf("restorer called %d times, want 0", restorer.calls)
	}
	if item.Count != 5 || itemRepo.updated != nil || itemRepo.deleted != 0 {
		t.Error("no consumption expected on no-op")
	}
}

func TestPotionHandler_UnknownEffectNotConsumed(t *testing.T) {
	// Skill exists in template but the loader does not resolve it (e.g. buff skill).
	h, restorer, itemRepo := newPotionTest(map[[2]int]registry.SkillEffect{})
	repository := &fakeRepo{item: itemRepo}

	item := &models.CharacterItem{ObjectID: 503, ItemID: 99, OwnerID: 7, Count: 2}
	tmpl := &registry.ItemTemplate{
		ID: 99, Handler: "ItemSkills",
		ItemSkills: []registry.ItemSkill{{ID: 1234, Level: 1}},
	}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: repository})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed || restorer.calls != 0 || item.Count != 2 {
		t.Error("unresolved skill must be a no-op (not consumed, no restore)")
	}
}
