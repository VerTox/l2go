package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// fakeItemHandler is a test double recording invocations.
type fakeItemHandler struct {
	called   int
	consumed bool
	err      error
	lastUse  ItemUseContext
}

func (f *fakeItemHandler) UseItem(_ context.Context, use ItemUseContext) (bool, error) {
	f.called++
	f.lastUse = use
	return f.consumed, f.err
}

func TestItemHandlerRegistry_RegisterAndGet(t *testing.T) {
	r := NewItemHandlerRegistry()
	h := &fakeItemHandler{}

	r.Register("SoulShots", h)

	got, ok := r.Get("SoulShots")
	if !ok || got != h {
		t.Fatalf("Get(SoulShots) = %v, %v; want handler, true", got, ok)
	}
	if r.Count() != 1 {
		t.Errorf("Count() = %d, want 1", r.Count())
	}

	if _, ok := r.Get("Unknown"); ok {
		t.Error("Get(Unknown) returned ok=true, want false")
	}
	if _, ok := r.Get(""); ok {
		t.Error("Get(\"\") returned ok=true, want false")
	}
}

func TestItemHandlerRegistry_IgnoresEmptyAndNil(t *testing.T) {
	r := NewItemHandlerRegistry()
	r.Register("", &fakeItemHandler{})
	r.Register("X", nil)
	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0 (empty name / nil handler ignored)", r.Count())
	}
}

func newInventoryUCForTest() *InventoryUseCase {
	// repo is nil on purpose: useNonEquipItem must not touch it directly, it only
	// passes it through into ItemUseContext.
	return &InventoryUseCase{itemHandlers: NewItemHandlerRegistry()}
}

func TestUseNonEquipItem_NoHandlerIsNoOp(t *testing.T) {
	uc := newInventoryUCForTest()
	item := &models.CharacterItem{ObjectID: 100, ItemID: 1463, OwnerID: 7}

	// Handler declared but not registered.
	tmpl := &registry.ItemTemplate{ID: 1463, Name: "Soulshot", Handler: "SoulShots"}
	res, err := uc.useNonEquipItem(context.Background(), 7, item, tmpl)
	if err != nil {
		t.Fatalf("unexpected error for unregistered handler: %v", err)
	}
	if res == nil || res.Success {
		t.Errorf("res = %+v, want non-nil with Success=false (no-op)", res)
	}

	// No handler name at all.
	tmpl2 := &registry.ItemTemplate{ID: 999, Name: "Plain", Handler: ""}
	res2, err := uc.useNonEquipItem(context.Background(), 7, item, tmpl2)
	if err != nil {
		t.Fatalf("unexpected error for empty handler: %v", err)
	}
	if res2 == nil || res2.Success {
		t.Errorf("res2 = %+v, want non-nil with Success=false (no-op)", res2)
	}
}

func TestUseNonEquipItem_DispatchesToHandler(t *testing.T) {
	uc := newInventoryUCForTest()
	h := &fakeItemHandler{consumed: true}
	uc.ItemHandlers().Register("SoulShots", h)

	item := &models.CharacterItem{ObjectID: 100, ItemID: 1463, OwnerID: 7}
	tmpl := &registry.ItemTemplate{ID: 1463, Name: "Soulshot", Handler: "SoulShots"}

	res, err := uc.useNonEquipItem(context.Background(), 7, item, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.called != 1 {
		t.Errorf("handler called %d times, want 1", h.called)
	}
	if !res.Success {
		t.Errorf("res.Success = false, want true (consumed propagated)")
	}
	if h.lastUse.CharID != 7 || h.lastUse.Item != item || h.lastUse.Template != tmpl {
		t.Errorf("handler received wrong use context: %+v", h.lastUse)
	}
}

func TestUseNonEquipItem_NotConsumed(t *testing.T) {
	uc := newInventoryUCForTest()
	uc.ItemHandlers().Register("SoulShots", &fakeItemHandler{consumed: false})

	item := &models.CharacterItem{ObjectID: 100, ItemID: 1463, OwnerID: 7}
	tmpl := &registry.ItemTemplate{ID: 1463, Handler: "SoulShots"}

	res, err := uc.useNonEquipItem(context.Background(), 7, item, tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success {
		t.Error("res.Success = true, want false when handler did not consume")
	}
}

func TestUseNonEquipItem_PropagatesError(t *testing.T) {
	uc := newInventoryUCForTest()
	wantErr := errors.New("boom")
	uc.ItemHandlers().Register("SoulShots", &fakeItemHandler{err: wantErr})

	item := &models.CharacterItem{ObjectID: 100, ItemID: 1463, OwnerID: 7}
	tmpl := &registry.ItemTemplate{ID: 1463, Handler: "SoulShots"}

	_, err := uc.useNonEquipItem(context.Background(), 7, item, tmpl)
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wrapped %v", err, wantErr)
	}
}

func TestItemHandlerRegistry_ConcurrentAccess(t *testing.T) {
	r := NewItemHandlerRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			r.Register("SoulShots", &fakeItemHandler{})
		}()
		go func() {
			defer wg.Done()
			_, _ = r.Get("SoulShots")
			_ = r.Count()
		}()
	}
	wg.Wait()
}
