package registry

import "sync"

// AutoShotRegistry tracks, per character, the set of shot item ids toggled for
// auto-use via RequestAutoSoulShot. It mirrors L2PcInstance._activeSoulShots: an
// in-memory set, NOT persisted — reset on relog (Clear on RemovePlayer), exactly
// like retail. Safe for concurrent use (the toggle handler runs on the connection
// goroutine; the recharge sink reads it from a background goroutine).
type AutoShotRegistry struct {
	mu     sync.RWMutex
	byChar map[int32]map[int32]struct{} // charID -> shot itemID -> active
}

// NewAutoShotRegistry creates an empty auto-shot registry.
func NewAutoShotRegistry() *AutoShotRegistry {
	return &AutoShotRegistry{byChar: make(map[int32]map[int32]struct{})}
}

// Global auto-shot registry instance (mirrors GetChargedShotRegistry pattern).
var autoShots = NewAutoShotRegistry()

// GetAutoShotRegistry returns the global auto-shot registry.
func GetAutoShotRegistry() *AutoShotRegistry { return autoShots }

// Enable marks a shot item id as auto-used for a character.
func (r *AutoShotRegistry) Enable(charID, itemID int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.byChar[charID]
	if m == nil {
		m = make(map[int32]struct{})
		r.byChar[charID] = m
	}
	m[itemID] = struct{}{}
}

// Disable removes a shot item id from a character's auto-used set.
func (r *AutoShotRegistry) Disable(charID, itemID int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.byChar[charID]
	if m == nil {
		return
	}
	delete(m, itemID)
	if len(m) == 0 {
		delete(r.byChar, charID)
	}
}

// IsActive reports whether the shot item id is auto-used for the character.
func (r *AutoShotRegistry) IsActive(charID, itemID int32) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.byChar[charID][itemID]
	return ok
}

// HasAny reports whether the character has any active auto-shot (cheap check used
// by the combat loop before enqueueing a recharge).
func (r *AutoShotRegistry) HasAny(charID int32) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byChar[charID]) > 0
}

// List returns the character's active auto-shot item ids (order unspecified).
func (r *AutoShotRegistry) List(charID int32) []int32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := r.byChar[charID]
	if len(m) == 0 {
		return nil
	}
	out := make([]int32, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out
}

// Clear drops all auto-shots for a character (on logout/relog, so the set resets
// like retail).
func (r *AutoShotRegistry) Clear(charID int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byChar, charID)
}
