package usecase

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// --- test doubles ---

// fakeSkillTemplates resolves only the (id,level) pairs it was seeded with.
type fakeSkillTemplates struct {
	known map[[2]int]bool
}

func (f *fakeSkillTemplates) GetSkill(id, level int) *models.Skill {
	if f.known[[2]int{id, level}] {
		return &models.Skill{ID: id, Level: level}
	}
	return nil
}

// recordingCaster captures CastItemSkill calls.
type recordingCaster struct {
	calls []struct{ charID, skillID, level int32 }
}

func (r *recordingCaster) CastItemSkill(charID, skillID, level int32) {
	r.calls = append(r.calls, struct{ charID, skillID, level int32 }{charID, skillID, level})
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

func newItemSkillTest(known map[[2]int]bool) (*ItemSkillHandler, *recordingCaster, *fakeItemRepo) {
	caster := &recordingCaster{}
	itemRepo := &fakeItemRepo{}
	h := NewItemSkillHandler(&fakeSkillTemplates{known: known}, caster)
	return h, caster, itemRepo
}

func TestItemSkillHandler_CastsAndConsumes(t *testing.T) {
	h, caster, itemRepo := newItemSkillTest(map[[2]int]bool{{2037, 1}: true})
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
	// The linked skill is cast once through the engine.
	if len(caster.calls) != 1 || caster.calls[0] != (struct{ charID, skillID, level int32 }{7, 2037, 1}) {
		t.Errorf("caster.calls = %+v, want one cast {7,2037,1}", caster.calls)
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

func TestItemSkillHandler_LastItemDeleted(t *testing.T) {
	h, _, itemRepo := newItemSkillTest(map[[2]int]bool{{10001, 1}: true})
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

func TestItemSkillHandler_NoSkillsIsNoOp(t *testing.T) {
	h, caster, itemRepo := newItemSkillTest(nil)
	repository := &fakeRepo{item: itemRepo}

	item := &models.CharacterItem{ObjectID: 502, ItemID: 1, OwnerID: 7, Count: 5}
	tmpl := &registry.ItemTemplate{ID: 1, Handler: "ItemSkills"} // no ItemSkills

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: repository})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false when no skills declared")
	}
	if len(caster.calls) != 0 {
		t.Errorf("caster called %d times, want 0", len(caster.calls))
	}
	if item.Count != 5 || itemRepo.updated != nil || itemRepo.deleted != 0 {
		t.Error("no consumption expected on no-op")
	}
}

func TestItemSkillHandler_UnknownSkillNotConsumed(t *testing.T) {
	// Skill declared in the template but not resolvable in the datapack → no-op.
	h, caster, itemRepo := newItemSkillTest(map[[2]int]bool{})
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
	if consumed || len(caster.calls) != 0 || item.Count != 2 {
		t.Error("unresolved skill must be a no-op (not consumed, not cast)")
	}
}
