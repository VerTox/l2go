package usecase

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// --- test doubles ---

// extractFakeItemRepo implements just enough of repo.ItemRepository for the
// extractable handler: consuming the box (Update/Delete) and giving rewards
// (FindStackableItem/Create/Update). Created items get incrementing object ids.
type extractFakeItemRepo struct {
	repo.ItemRepository // embedded: unimplemented methods panic if called

	// stackable inventory keyed by itemID -> existing stack (nil = none)
	existing map[int32]*models.CharacterItem

	created  []*models.CharacterItem
	updated  []*models.CharacterItem
	deleted  []int32
	nextOID  int32
}

func (f *extractFakeItemRepo) FindStackableItem(_ context.Context, _ int32, itemID int32, _ models.ItemLocation) (*models.CharacterItem, error) {
	if f.existing == nil {
		return nil, nil
	}
	return f.existing[itemID], nil
}

func (f *extractFakeItemRepo) Create(_ context.Context, item *models.CharacterItem) error {
	if f.nextOID == 0 {
		f.nextOID = 1000
	}
	f.nextOID++
	item.ObjectID = f.nextOID
	f.created = append(f.created, item)
	return nil
}

func (f *extractFakeItemRepo) Update(_ context.Context, item *models.CharacterItem) error {
	f.updated = append(f.updated, item)
	return nil
}

func (f *extractFakeItemRepo) Delete(_ context.Context, objectID int32) error {
	f.deleted = append(f.deleted, objectID)
	return nil
}

// extractFakeDB implements repo.DatabaseRepository, exposing only Item().
type extractFakeDB struct {
	repo.DatabaseRepository
	item *extractFakeItemRepo
}

func (f *extractFakeDB) Item() repo.ItemRepository { return f.item }

// scriptedRNG returns queued values in order; when exhausted it returns 0.
// Signature matches the injected rng: rng(n) in [0,n).
type scriptedRNG struct {
	vals []int
	i    int
}

func (r *scriptedRNG) next(_ int) int {
	if r.i >= len(r.vals) {
		return 0
	}
	v := r.vals[r.i]
	r.i++
	return v
}

// newExtractTest wires a handler with a scripted rng and a template lookup that
// reports stackability per item id.
func newExtractTest(rng []int, stackable map[int32]bool) (*ExtractableItemsHandler, *extractFakeItemRepo) {
	itemRepo := &extractFakeItemRepo{existing: map[int32]*models.CharacterItem{}}
	h := NewExtractableItemsHandler()
	sr := &scriptedRNG{vals: rng}
	h.rng = sr.next
	h.template = func(itemID int32) *registry.ItemTemplate {
		return &registry.ItemTemplate{ID: itemID, Stackable: stackable[itemID]}
	}
	return h, itemRepo
}

func extractCtx(charID int32, item *models.CharacterItem, tmpl *registry.ItemTemplate, r repo.DatabaseRepository) (ItemUseContext, *[]ChangedItem) {
	var emitted []ChangedItem
	uc := ItemUseContext{
		CharID:   charID,
		Item:     item,
		Template: tmpl,
		Repo:     r,
		Emit:     func(ci ChangedItem) { emitted = append(emitted, ci) },
	}
	return uc, &emitted
}

func TestExtractable_GuaranteedDrop_NewStack(t *testing.T) {
	// One product, 100% chance, fixed count 5, reward stackable, no existing stack.
	h, itemRepo := newExtractTest([]int{0}, map[int32]bool{13010: true})
	db := &extractFakeDB{item: itemRepo}

	box := &models.CharacterItem{ObjectID: 500, ItemID: 13277, OwnerID: 7, Count: 3}
	tmpl := &registry.ItemTemplate{
		ID:            13277,
		Handler:       "ExtractableItems",
		Stackable:     true,
		CapsuledItems: []registry.ExtractableProduct{{ID: 13010, Min: 5, Max: 5, Chance: 100000}},
	}
	uc, emitted := extractCtx(7, box, tmpl, db)

	consumed, err := h.UseItem(context.Background(), uc)
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true")
	}
	// box: 3 -> 2, updated not deleted
	if box.Count != 2 {
		t.Errorf("box.Count = %d, want 2", box.Count)
	}
	// reward created as new stack of 5
	if len(itemRepo.created) != 1 || itemRepo.created[0].ItemID != 13010 || itemRepo.created[0].Count != 5 {
		t.Fatalf("created = %+v, want single 13010 x5", itemRepo.created)
	}
	if itemRepo.created[0].Loc != string(models.LocInventory) {
		t.Errorf("reward Loc = %q, want INVENTORY", itemRepo.created[0].Loc)
	}
	if len(*emitted) != 1 || (*emitted)[0].UpdateType != 1 || (*emitted)[0].Item.ItemID != 13010 {
		t.Errorf("emitted = %+v, want single ADD 13010", *emitted)
	}
}

func TestExtractable_ChanceMiss_NothingCreated(t *testing.T) {
	// 30% chance, roll 50000 > 30000 -> miss. Box still consumed, consumed=true.
	h, itemRepo := newExtractTest([]int{50000}, map[int32]bool{13011: true})
	db := &extractFakeDB{item: itemRepo}

	box := &models.CharacterItem{ObjectID: 501, ItemID: 13277, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{
		ID:            13277,
		Handler:       "ExtractableItems",
		Stackable:     true,
		CapsuledItems: []registry.ExtractableProduct{{ID: 13011, Min: 1, Max: 1, Chance: 30000}},
	}
	uc, emitted := extractCtx(7, box, tmpl, db)

	consumed, err := h.UseItem(context.Background(), uc)
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true (box destroyed even on empty roll)")
	}
	// last box deleted
	if box.Count != 0 || len(itemRepo.deleted) != 1 || itemRepo.deleted[0] != 501 {
		t.Errorf("box not deleted: count=%d deleted=%v", box.Count, itemRepo.deleted)
	}
	if len(itemRepo.created) != 0 || len(*emitted) != 0 {
		t.Errorf("nothing should be created on miss: created=%+v emitted=%+v", itemRepo.created, *emitted)
	}
}

func TestExtractable_CountRange(t *testing.T) {
	// Min=2 Max=6 -> range width 5. rng: [0 (hit), 2 (offset)] => amount = 2+2 = 4.
	h, itemRepo := newExtractTest([]int{0, 2}, map[int32]bool{13010: true})
	db := &extractFakeDB{item: itemRepo}

	box := &models.CharacterItem{ObjectID: 502, ItemID: 13277, OwnerID: 7, Count: 5}
	tmpl := &registry.ItemTemplate{
		ID:            13277,
		Stackable:     true,
		CapsuledItems: []registry.ExtractableProduct{{ID: 13010, Min: 2, Max: 6, Chance: 100000}},
	}
	uc, _ := extractCtx(7, box, tmpl, db)

	if _, err := h.UseItem(context.Background(), uc); err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if len(itemRepo.created) != 1 || itemRepo.created[0].Count != 4 {
		t.Fatalf("created = %+v, want count 4", itemRepo.created)
	}
}

func TestExtractable_MultipleRewards(t *testing.T) {
	// Two products both 100%: rng [0 (hit A), 0 (hit B)]. Both min==max so no count roll.
	h, itemRepo := newExtractTest([]int{0, 0}, map[int32]bool{13010: true, 13011: true})
	db := &extractFakeDB{item: itemRepo}

	box := &models.CharacterItem{ObjectID: 503, ItemID: 13277, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{
		ID:        13277,
		Stackable: true,
		CapsuledItems: []registry.ExtractableProduct{
			{ID: 13010, Min: 5, Max: 5, Chance: 100000},
			{ID: 13011, Min: 3, Max: 3, Chance: 100000},
		},
	}
	uc, emitted := extractCtx(7, box, tmpl, db)

	if _, err := h.UseItem(context.Background(), uc); err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if len(itemRepo.created) != 2 || len(*emitted) != 2 {
		t.Fatalf("want 2 rewards, created=%+v emitted=%+v", itemRepo.created, *emitted)
	}
	if itemRepo.created[0].ItemID != 13010 || itemRepo.created[0].Count != 5 {
		t.Errorf("reward0 = %+v, want 13010 x5", itemRepo.created[0])
	}
	if itemRepo.created[1].ItemID != 13011 || itemRepo.created[1].Count != 3 {
		t.Errorf("reward1 = %+v, want 13011 x3", itemRepo.created[1])
	}
}

func TestExtractable_StackIntoExisting(t *testing.T) {
	h, itemRepo := newExtractTest([]int{0}, map[int32]bool{13010: true})
	// existing stack of 10 in inventory
	itemRepo.existing[13010] = &models.CharacterItem{ObjectID: 900, ItemID: 13010, OwnerID: 7, Count: 10, Loc: string(models.LocInventory)}
	db := &extractFakeDB{item: itemRepo}

	box := &models.CharacterItem{ObjectID: 504, ItemID: 13277, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{
		ID:            13277,
		Stackable:     true,
		CapsuledItems: []registry.ExtractableProduct{{ID: 13010, Min: 5, Max: 5, Chance: 100000}},
	}
	uc, emitted := extractCtx(7, box, tmpl, db)

	if _, err := h.UseItem(context.Background(), uc); err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if len(itemRepo.created) != 0 {
		t.Errorf("no new item should be created when stacking, got %+v", itemRepo.created)
	}
	if itemRepo.existing[13010].Count != 15 {
		t.Errorf("existing stack count = %d, want 15", itemRepo.existing[13010].Count)
	}
	// existing stack updated (box delete + reward update)
	foundReward := false
	for _, u := range itemRepo.updated {
		if u.ObjectID == 900 && u.Count == 15 {
			foundReward = true
		}
	}
	if !foundReward {
		t.Errorf("expected reward stack Update to 15, updated=%+v", itemRepo.updated)
	}
	if len(*emitted) != 1 || (*emitted)[0].UpdateType != 2 || (*emitted)[0].Item.ObjectID != 900 {
		t.Errorf("emitted = %+v, want single MODIFY of obj 900", *emitted)
	}
}

func TestExtractable_NonStackableSeparateObjects(t *testing.T) {
	// reward non-stackable, amount 3 -> 3 separate objects of count 1.
	h, itemRepo := newExtractTest([]int{0}, map[int32]bool{40: false})
	db := &extractFakeDB{item: itemRepo}

	box := &models.CharacterItem{ObjectID: 505, ItemID: 13277, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{
		ID:            13277,
		Stackable:     true,
		CapsuledItems: []registry.ExtractableProduct{{ID: 40, Min: 3, Max: 3, Chance: 100000}},
	}
	uc, emitted := extractCtx(7, box, tmpl, db)

	if _, err := h.UseItem(context.Background(), uc); err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if len(itemRepo.created) != 3 {
		t.Fatalf("want 3 separate objects, got %+v", itemRepo.created)
	}
	for i, it := range itemRepo.created {
		if it.ItemID != 40 || it.Count != 1 {
			t.Errorf("created[%d] = %+v, want 40 x1", i, it)
		}
	}
	if len(*emitted) != 3 {
		t.Errorf("want 3 ADD emits, got %+v", *emitted)
	}
}

func TestExtractable_NoCapsuledItemsIsNoOp(t *testing.T) {
	h, itemRepo := newExtractTest(nil, nil)
	db := &extractFakeDB{item: itemRepo}

	box := &models.CharacterItem{ObjectID: 506, ItemID: 999, OwnerID: 7, Count: 4}
	tmpl := &registry.ItemTemplate{ID: 999, Handler: "ExtractableItems"} // no CapsuledItems
	uc, emitted := extractCtx(7, box, tmpl, db)

	consumed, err := h.UseItem(context.Background(), uc)
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false when no capsuled items")
	}
	if box.Count != 4 || len(itemRepo.deleted) != 0 || len(itemRepo.created) != 0 || len(*emitted) != 0 {
		t.Error("no-op must not touch inventory")
	}
}
