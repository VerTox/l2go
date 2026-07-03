package registry

import "sync"

// ShotType identifies a charged-shot kind held on a weapon instance.
// Mirrors L2J's ShotType enum (SOULSHOTS / SPIRITSHOTS / BLESSED_SPIRITSHOTS /
// FISH_SOULSHOTS …). The weapon-facing shots are modelled here; the beast
// (summon) and fish (fishing-rod) charge types are intentionally omitted until
// the pet/fishing systems exist (l2go-82b parks them).
type ShotType int

const (
	ShotSoulshot ShotType = iota
	ShotSpiritshot
	// ShotBlessedSpiritshot is a separate charge from ShotSpiritshot: a weapon
	// can hold a blessed spiritshot charge independently, mirroring L2J's
	// BLESSED_SPIRITSHOTS ("it can be charged over SpiritShot").
	ShotBlessedSpiritshot
)

// ChargedShotRegistry tracks, per weapon item instance (its object id), which
// shot types are currently charged. This mirrors L2J's per-instance runtime
// state on L2ItemInstance (_chargedSoulshot / _chargedSpiritshot): a charged
// weapon spends the charge on its next attack. The state is purely in-memory
// and is NOT persisted to the database — charges are lost on relog, exactly
// like retail.
type ChargedShotRegistry struct {
	mu sync.RWMutex
	// weaponObjectID -> shot -> grade id. Presence of the (weapon,shot) key means
	// charged; the value is the weapon's grade id (getItemGradeSPlus) used only to
	// tint the soulshot hit visual. Grade 0 (no-grade weapon) is a valid charged
	// state, so charged-ness is presence, never the value.
	charged map[int32]map[ShotType]int
}

// NewChargedShotRegistry creates an empty charged-shot registry.
func NewChargedShotRegistry() *ChargedShotRegistry {
	return &ChargedShotRegistry{
		charged: make(map[int32]map[ShotType]int),
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
	m := r.charged[weaponObjectID]
	if m == nil {
		return false
	}
	_, ok := m[shot]
	return ok
}

// ChargedGrade returns the weapon grade id stored with the charge (for the
// soulshot hit visual), or 0 when the weapon holds no charge of that shot type.
func (r *ChargedShotRegistry) ChargedGrade(weaponObjectID int32, shot ShotType) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.charged[weaponObjectID][shot] // 0 when absent
}

// Charge marks a shot type charged on a weapon instance and records its grade id.
func (r *ChargedShotRegistry) Charge(weaponObjectID int32, shot ShotType, grade int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.charged[weaponObjectID]
	if m == nil {
		m = make(map[ShotType]int)
		r.charged[weaponObjectID] = m
	}
	m[shot] = grade
}

// SetCharged charges (grade 0) or spends the charge of a shot type on a weapon
// instance. Clearing the last charge removes the weapon's entry to keep the map
// compact. Use Charge to record a grade.
func (r *ChargedShotRegistry) SetCharged(weaponObjectID int32, shot ShotType, charged bool) {
	if charged {
		r.Charge(weaponObjectID, shot, 0)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
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
