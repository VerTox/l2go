package registry

import (
	"sync"
	"time"
)

// itemReuseStamp is a single per-item reuse cooldown, mirroring L2J's TimeStamp
// (model/TimeStamp.java): the moment the cooldown ends, the total configured
// reuse duration and the shared-reuse-group the item belongs to.
type itemReuseStamp struct {
	end   time.Time
	total time.Duration
	group int
}

// ItemReuseRegistry stores per-character item reuse cooldowns in memory, keyed by
// item object id — the analogue of L2Character._reuseTimeStampsItems. It is NOT
// persisted: like retail, cooldowns reset on relog (dropped via Clear when the
// player is removed from the world). Safe for concurrent use.
type ItemReuseRegistry struct {
	mu     sync.Mutex
	byChar map[int32]map[int32]itemReuseStamp // charID -> item objectID -> stamp
}

// NewItemReuseRegistry creates an empty item reuse registry.
func NewItemReuseRegistry() *ItemReuseRegistry {
	return &ItemReuseRegistry{byChar: make(map[int32]map[int32]itemReuseStamp)}
}

// Global item reuse registry instance (mirrors GetChargedShotRegistry pattern).
var itemReuse = NewItemReuseRegistry()

// GetItemReuseRegistry returns the global item reuse registry.
func GetItemReuseRegistry() *ItemReuseRegistry { return itemReuse }

// Add records a reuse cooldown for an item instance ending at now+total. A
// non-positive total is ignored (no cooldown). Mirrors L2J addTimeStampItem.
func (r *ItemReuseRegistry) Add(charID, objectID int32, group int, total time.Duration, now time.Time) {
	if total <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.byChar[charID]
	if m == nil {
		m = make(map[int32]itemReuseStamp)
		r.byChar[charID] = m
	}
	m[objectID] = itemReuseStamp{end: now.Add(total), total: total, group: group}
}

// RemainingByItem returns the remaining cooldown for a specific item instance, or
// 0 if there is none or it has already elapsed. Mirrors getItemRemainingReuseTime.
func (r *ItemReuseRegistry) RemainingByItem(charID, objectID int32, now time.Time) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.byChar[charID][objectID]
	if !ok {
		return 0
	}
	if rem := st.end.Sub(now); rem > 0 {
		return rem
	}
	return 0
}

// RemainingByGroup returns the largest remaining cooldown among still-active items
// in the given shared reuse group, or 0. group<=0 always returns 0. Mirrors
// getReuseDelayOnGroup (which returns the first not-yet-passed group member).
func (r *ItemReuseRegistry) RemainingByGroup(charID int32, group int, now time.Time) time.Duration {
	if group <= 0 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var best time.Duration
	for _, st := range r.byChar[charID] {
		if st.group != group {
			continue
		}
		if rem := st.end.Sub(now); rem > best {
			best = rem
		}
	}
	return best
}

// Clear drops all cooldowns for a character (on logout/relog, so reuse resets like
// retail).
func (r *ItemReuseRegistry) Clear(charID int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byChar, charID)
}
