package usecase

import (
	"context"
	"sync"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// ItemUseContext carries the state an ItemHandler needs to act on a used item.
// It mirrors the arguments L2J passes to IItemHandler.useItem (playable + item).
type ItemUseContext struct {
	CharID   int32                   // owner character id
	Item     *models.CharacterItem   // the item instance being used
	Template *registry.ItemTemplate  // static template for the item
	Repo     repo.DatabaseRepository // repository access for handler side effects
}

// ItemHandler handles the "use" (double-click) of a non-equipment item.
// This mirrors L2J's IItemHandler: concrete handlers (SoulShots, potions,
// enchant scrolls, etc.) implement this interface and are registered by name.
//
// UseItem returns consumed=true when the item was actually acted upon (so the
// caller may report success / consume a charge). A handler that decides to do
// nothing should return consumed=false with a nil error.
type ItemHandler interface {
	UseItem(ctx context.Context, use ItemUseContext) (consumed bool, err error)
}

// ItemHandlerRegistry is a thread-safe registry of ItemHandlers keyed by the
// handler name declared in the item template (template.Handler). It mirrors
// L2J's ItemHandler datatable, which maps handler simple-name -> handler.
type ItemHandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]ItemHandler
}

// NewItemHandlerRegistry creates an empty item handler registry.
func NewItemHandlerRegistry() *ItemHandlerRegistry {
	return &ItemHandlerRegistry{
		handlers: make(map[string]ItemHandler),
	}
}

// Register associates a handler with a name (e.g. "SoulShots"). An empty name
// is ignored so that items without a handler can never be matched.
func (r *ItemHandlerRegistry) Register(name string, handler ItemHandler) {
	if name == "" || handler == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

// Get returns the handler registered under name, if any.
func (r *ItemHandlerRegistry) Get(name string) (ItemHandler, bool) {
	if name == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}

// Count returns the number of registered handlers.
func (r *ItemHandlerRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}
