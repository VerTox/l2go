package registry

import "sync"

// ShotType identifies a charged-shot kind held on a weapon instance.
// Mirrors L2J's ShotType enum (SOULSHOTS / SPIRITSHOTS / …). Only the two
// physical/magic base shots are modelled here; blessed/beast/fish variants are
// left for a follow-up (l2go-82b).
type ShotType int

const (
	ShotSoulshot ShotType = iota
	ShotSpiritshot
)

// ChargedShotRegistry tracks, per weapon item instance (its object id), which
// shot types are currently charged. This mirrors L2J's per-instance runtime
// state on L2ItemInstance (_chargedSoulshot / _chargedSpiritshot): a charged
// weapon spends the charge on its next attack. The state is purely in-memory
// and is NOT persisted to the database — charges are lost on relog, exactly
// like retail.
type ChargedShotRegistry struct {
	mu      sync.RWMutex
	charged map[int32]map[ShotType]bool // weaponObjectID -> shot -> charged
}

// NewChargedShotRegistry creates an empty charged-shot registry.
func NewChargedShotRegistry() *ChargedShotRegistry {
	return &ChargedShotRegistry{
		charged: make(map[int32]map[ShotType]bool),
	}
}

// Global singleton so the item-handler layer (which charges shots) and the game
// loop (which will later spend them in combat) share the same state.
var chargedShots = NewChargedShotRegistry()

// GetChargedShotRegistry returns the global charged-shot registry.
func GetChargedShotRegistry() *ChargedShotRegistry { return chargedShots }

// IsCharged reports whether the given weapon instance currently holds a charge
// of the given shot type.
func (r *ChargedShotRegistry) IsCharged(weaponObjectID int32, shot ShotType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.charged[weaponObjectID][shot]
}

// SetCharged sets (or clears) the charge of a shot type on a weapon instance.
// Clearing the last charge removes the weapon's entry to keep the map compact.
func (r *ChargedShotRegistry) SetCharged(weaponObjectID int32, shot ShotType, charged bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if charged {
		m := r.charged[weaponObjectID]
		if m == nil {
			m = make(map[ShotType]bool)
			r.charged[weaponObjectID] = m
		}
		m[shot] = true
		return
	}

	m := r.charged[weaponObjectID]
	if m == nil {
		return
	}
	delete(m, shot)
	if len(m) == 0 {
		delete(r.charged, weaponObjectID)
	}
}

// Clear drops all charged state for a weapon instance (e.g. on unequip or when
// the item is destroyed).
func (r *ChargedShotRegistry) Clear(weaponObjectID int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.charged, weaponObjectID)
}
