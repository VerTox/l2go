package registry

import "sync"

// ActiveEnchant holds the enchant scroll a player has "armed" by using it, while
// the client is choosing which item to enchant. Mirrors L2J's per-player
// _activeEnchantItemId / scroll object on L2PcInstance. Purely in-memory: the
// arming is transient and never persisted (lost on relog, like retail).
type ActiveEnchant struct {
	ScrollObjectID int32 // inventory object id of the scroll being used
	ScrollItemID   int32 // scroll template id (sent in ChooseInventoryItem)
}

// EnchantStateRegistry tracks, per character, the currently-armed enchant scroll
// between the UseItem (ChooseInventoryItem) step and the RequestEnchantItem step.
type EnchantStateRegistry struct {
	mu     sync.RWMutex
	active map[int32]ActiveEnchant // charID -> armed scroll
}

// NewEnchantStateRegistry creates an empty enchant-state registry.
func NewEnchantStateRegistry() *EnchantStateRegistry {
	return &EnchantStateRegistry{active: make(map[int32]ActiveEnchant)}
}

// Global singleton shared by the item-handler layer (which arms scrolls) and the
// RequestEnchantItem handler (which consumes the arming).
var enchantState = NewEnchantStateRegistry()

// GetEnchantStateRegistry returns the global enchant-state registry.
func GetEnchantStateRegistry() *EnchantStateRegistry { return enchantState }

// SetActive arms an enchant scroll for a character, replacing any prior arming.
func (r *EnchantStateRegistry) SetActive(charID int32, a ActiveEnchant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active[charID] = a
}

// GetActive returns the armed scroll for a character, if any.
func (r *EnchantStateRegistry) GetActive(charID int32) (ActiveEnchant, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.active[charID]
	return a, ok
}

// HasActive reports whether a character currently has an armed scroll.
func (r *EnchantStateRegistry) HasActive(charID int32) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.active[charID]
	return ok
}

// Clear removes any armed scroll for a character.
func (r *EnchantStateRegistry) Clear(charID int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.active, charID)
}
