package registry

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// WorldObject represents an object in the game world
type WorldObject struct {
	ID       int32                  `json:"id"`
	Name     string                 `json:"name"`
	Type     WorldObjectType        `json:"type"`
	Position models.Position        `json:"position"`
	Data     map[string]interface{} `json:"data,omitempty"` // Flexible data storage
}

// WorldObjectType defines the type of objects in the world
type WorldObjectType int

const (
	ObjectTypePlayer WorldObjectType = iota
	ObjectTypeNPC
	ObjectTypeItem
	ObjectTypeDoor
	ObjectTypeSpawn
)

// PlayerWorldState represents a player's state in the world
type PlayerWorldState struct {
	CharID      int32           `json:"char_id"`
	AccountName string          `json:"account_name"`
	Character   *models.Character `json:"character"`
	Position    models.Position `json:"position"`
	Heading     int32           `json:"heading"`
	Online      bool            `json:"online"`
	InCombat    bool            `json:"in_combat"`
	LastUpdate  time.Time       `json:"last_update"`

	// PvPFlagUntil — время, до которого игрок PvP-флагнут (фиолетовое имя,
	// атакуем без Ctrl). Loop-owned. Ноль = не флагнут. (l2go-fgz)
	PvPFlagUntil time.Time `json:"-"`
	
	// IsTeleporting is set while a teleport is in flight: the server has sent
	// TeleportToLocation and decayed the player, and is waiting for the client's
	// Appearing packet to re-establish visibility at the new position.
	IsTeleporting bool `json:"is_teleporting"`

	// Movement state
	IsMoving        bool            `json:"is_moving"`
	MoveStarted     time.Time       `json:"move_started,omitempty"`
	MoveStartPos    models.Position `json:"move_start_pos,omitempty"`    // Position when movement started
	MoveDestination models.Position `json:"move_destination,omitempty"`  // Target destination
	IsRunning       bool            `json:"is_running"`                  // L2J persistent run/walk state
	
	// World interaction
	TargetID    int32           `json:"target_id,omitempty"`

	// KnownSkills maps a learned skill id to its level (populated at world entry).
	// Read by the game loop to validate/resolve casts. Set once before the player
	// goes live; treated as read-only afterward for this phase.
	KnownSkills map[int32]int32 `json:"-"`

	// Casting holds the in-progress cast, or nil when the player is not casting.
	// Owned by the game loop goroutine.
	Casting *CastState `json:"-"`

	// PassiveMods are the character's passive-skill stat modifiers (l2go-9ep),
	// set at world entry. EquipMods are the equipped items' stat bonuses, refreshed
	// on equip/unequip. Both are combined with active-buff mods into Character.StatMods.
	PassiveMods []models.StatModifier `json:"-"`
	EquipMods   []models.StatModifier `json:"-"`

	// Effects holds the active continuous effects (buffs/debuffs/toggles, l2go-c8t).
	// Owned by the game loop goroutine.
	Effects models.CharEffectList `json:"-"`

	// Known objects (sent to client, used for visibility tracking)
	KnownNPCs map[int32]bool `json:"-"` // NPC objectIDs already sent to this client
	// KnownPlayers tracks other players already spawned to this client (CharInfo sent).
	// Owned exclusively by the game loop — only the loop goroutine reads/writes it, so
	// no locking is needed despite being shared world state. (l2go-23g)
	KnownPlayers map[int32]bool `json:"-"`

	// Session info
	SessionData map[string]interface{} `json:"session_data,omitempty"`
}

// RebuildStatMods recomputes Character.StatMods as the union of the character's
// passive-skill mods, equipped-item mods, and active-buff mods. It is the single
// source of truth for the stat-modifier layer, so every stat consumer (combat,
// UserInfo, CharInfo — whether built by the loop or a handler) sees the same value.
func (p *PlayerWorldState) RebuildStatMods() {
	if p.Character == nil {
		return
	}
	mods := make([]models.StatModifier, 0, len(p.PassiveMods)+len(p.EquipMods))
	mods = append(mods, p.PassiveMods...)
	mods = append(mods, p.EquipMods...)
	mods = append(mods, p.Effects.Mods()...)
	p.Character.StatMods = mods
}

// IsPvPFlagged reports whether the player currently carries a PvP flag.
func (p *PlayerWorldState) IsPvPFlagged(now time.Time) bool {
	return !p.PvPFlagUntil.IsZero() && now.Before(p.PvPFlagUntil)
}

// CastState is an in-progress skill cast, owned by the game loop. The unique ID
// lets a scheduled hit event detect that the cast was aborted or superseded (the
// player's current Casting.ID no longer matches) and do nothing.
type CastState struct {
	ID         int64
	SkillID    int32
	SkillLevel int32
	TargetID   int32
}

// WorldRegistry manages all world objects and player states
type WorldRegistry struct {
	mu      sync.RWMutex
	players map[int32]*PlayerWorldState    // charID -> state
	objects map[int32]*WorldObject         // objectID -> object
	npcs    map[int32]*models.NpcInstance  // objectID -> NPC instance

	// Spatial indexing (simple implementation). Keys are packed int64 region
	// coordinates (see packRegion) rather than "x,y" strings, so grid queries build
	// no per-call string garbage — the fixed ~6µs / dozens-of-allocs floor the
	// string keys imposed on every GetPlayersInRange/GetNPCsInRange call. (l2go-8hy)
	regions map[int64][]int32 // packRegion(rx,ry) -> []objectIDs

	// Reverse who-targets-what index, so an object's updates broadcast to exactly
	// its targeters in O(targeters) instead of scanning all players. (l2go-45b)
	targets *targetIndex
}

// NewWorldRegistry creates a new world registry
func NewWorldRegistry() *WorldRegistry {
	return &WorldRegistry{
		players: make(map[int32]*PlayerWorldState),
		objects: make(map[int32]*WorldObject),
		npcs:    make(map[int32]*models.NpcInstance),
		regions: make(map[int64][]int32),
		targets: newTargetIndex(),
	}
}

// Player Management

// AddPlayer adds a player to the world
func (wr *WorldRegistry) AddPlayer(ctx context.Context, char *models.Character) error {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	
	state := &PlayerWorldState{
		CharID:      char.ID,
		AccountName: char.AccountName,
		Character:   char,
		Position:    char.Position,
		Heading:     0, // TODO: Load from character data
		Online:      true,
		InCombat:    false,
		LastUpdate:  time.Now(),
		IsMoving:    false,
		IsRunning:   true, // L2J default: players start in running mode
		KnownNPCs:   make(map[int32]bool),
		KnownPlayers: make(map[int32]bool),
		SessionData: make(map[string]interface{}),
	}
	
	wr.players[char.ID] = state
	
	// Add to world objects
	obj := &WorldObject{
		ID:       char.ID,
		Name:     char.Name,
		Type:     ObjectTypePlayer,
		Position: char.Position,
		Data: map[string]interface{}{
			"account": char.AccountName,
			"level":   char.Level,
			"race":    char.Race,
			"class":   char.ClassID,
		},
	}
	wr.objects[char.ID] = obj
	
	// Add to spatial index
	regionKey := wr.getRegionKey(char.Position.X, char.Position.Y)
	wr.regions[regionKey] = append(wr.regions[regionKey], char.ID)
	
	log.Ctx(ctx).Info().
		Int32("char_id", char.ID).
		Str("name", char.Name).
		Str("account", char.AccountName).
		Int("x", char.Position.X).
		Int("y", char.Position.Y).
		Int("z", char.Position.Z).
		Msg("Player added to world")
	
	return nil
}

// RemovePlayer removes a player from the world
func (wr *WorldRegistry) RemovePlayer(ctx context.Context, charID int32) error {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	
	state, exists := wr.players[charID]
	if !exists {
		return nil // Already removed
	}
	
	// Remove from spatial index
	regionKey := wr.getRegionKey(state.Position.X, state.Position.Y)
	wr.removeFromRegion(regionKey, charID)

	// Remove from maps
	delete(wr.players, charID)
	delete(wr.objects, charID)

	// Drop from the reverse target index both ways: as a targeter (unlink its own
	// target) and as a target (drop anyone aiming at this now-gone player). (l2go-45b)
	wr.targets.set(charID, 0)
	wr.targets.dropTarget(charID)

	// Drop in-memory item reuse cooldowns so they reset on relog, like retail.
	GetItemReuseRegistry().Clear(charID)

	// Drop active auto-soulshots so they reset on relog (L2J does not persist the
	// _activeSoulShots set).
	GetAutoShotRegistry().Clear(charID)

	log.Ctx(ctx).Info().
		Int32("char_id", charID).
		Str("account", state.AccountName).
		Msg("Player removed from world")

	return nil
}

// GetPlayer retrieves a player's world state
func (wr *WorldRegistry) GetPlayer(charID int32) (*PlayerWorldState, bool) {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	
	state, exists := wr.players[charID]
	return state, exists
}

// GetPlayerByAccount retrieves a player by account name
func (wr *WorldRegistry) GetPlayerByAccount(accountName string) (*PlayerWorldState, bool) {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	
	for _, state := range wr.players {
		if state.AccountName == accountName {
			return state, true
		}
	}
	
	return nil, false
}

// GetPlayerByName returns the online player with the given character name.
// Matching is case-insensitive, mirroring L2J's name lookup (character names are
// unique case-insensitively). Used for TELL chat routing.
func (wr *WorldRegistry) GetPlayerByName(name string) (*PlayerWorldState, bool) {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	for _, state := range wr.players {
		if state.Character != nil && strings.EqualFold(state.Character.Name, name) {
			return state, true
		}
	}

	return nil, false
}

// UpdatePlayerPosition updates a player's position
func (wr *WorldRegistry) UpdatePlayerPosition(ctx context.Context, charID int32, newPos models.Position, heading int32) error {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	
	state, exists := wr.players[charID]
	if !exists {
		return ErrPlayerNotFound
	}
	
	oldRegionKey := wr.getRegionKey(state.Position.X, state.Position.Y)
	newRegionKey := wr.getRegionKey(newPos.X, newPos.Y)
	
	// Update position in both PlayerWorldState and Character model
	state.Position = newPos
	state.Heading = heading
	state.LastUpdate = time.Now()
	if state.Character != nil {
		state.Character.Position = newPos
	}
	
	// Update world object
	if obj, exists := wr.objects[charID]; exists {
		obj.Position = newPos
	}
	
	// Update spatial index if region changed
	if oldRegionKey != newRegionKey {
		wr.removeFromRegion(oldRegionKey, charID)
		wr.regions[newRegionKey] = append(wr.regions[newRegionKey], charID)
		
		log.Ctx(ctx).Debug().
			Int32("char_id", charID).
			Int64("old_region", oldRegionKey).
			Int64("new_region", newRegionKey).
			Msg("Player changed region")
	}
	
	return nil
}

// SetPlayerCombatState sets a player's combat state
func (wr *WorldRegistry) SetPlayerCombatState(charID int32, inCombat bool) error {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	
	state, exists := wr.players[charID]
	if !exists {
		return ErrPlayerNotFound
	}
	
	state.InCombat = inCombat
	state.LastUpdate = time.Now()

	return nil
}

// SetPlayerTarget records a player's current target (0 = none). It is the sole
// writer of PlayerWorldState.TargetID and keeps the reverse targeter index in
// sync, so broadcastToTargeters(objectID) can reach exactly the players aiming at
// objectID without scanning everyone. Safe to call from any goroutine. (l2go-45b)
func (wr *WorldRegistry) SetPlayerTarget(charID, targetID int32) {
	wr.mu.Lock()
	if state, exists := wr.players[charID]; exists {
		state.TargetID = targetID
	}
	wr.mu.Unlock()

	// Index has its own lock — updated outside wr.mu to keep the world critical
	// section short and avoid coupling the two locks.
	wr.targets.set(charID, targetID)
}

// GetPlayersTargeting returns a snapshot of charIDs whose current target is
// objectID. O(targeters), independent of total online count. (l2go-45b)
func (wr *WorldRegistry) GetPlayersTargeting(objectID int32) []int32 {
	return wr.targets.targetersOf(objectID)
}

// World Object Management

// GetObjectsInRange returns all objects within range of a position
func (wr *WorldRegistry) GetObjectsInRange(pos models.Position, radius int) []*WorldObject {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	
	var objects []*WorldObject
	
	// Simple implementation - check nearby regions
	regionKeys := wr.getNearbyRegions(pos.X, pos.Y, radius)
	
	seen := make(map[int32]bool)
	for _, regionKey := range regionKeys {
		if objectIDs, exists := wr.regions[regionKey]; exists {
			for _, objectID := range objectIDs {
				if seen[objectID] {
					continue
				}
				seen[objectID] = true
				
				if obj, exists := wr.objects[objectID]; exists {
					distance := wr.calculateDistance(pos, obj.Position)
					if distance <= radius {
						objects = append(objects, obj)
					}
				}
			}
		}
	}
	
	return objects
}

// GetPlayersInRange returns all players within range of a position.
//
// Backed by the region grid (regions): only the cells overlapping the radius are
// scanned, so the cost tracks players-near-the-point rather than total online N.
// Players are indexed into regions by AddPlayer/UpdatePlayerPosition/RemovePlayer,
// so the grid is always current. Mirrors GetNPCsInRange — the shared regions map
// holds both player charIDs and NPC objectIDs, so a wr.players lookup filters out
// NPC ids naturally. (l2go-g63)
func (wr *WorldRegistry) GetPlayersInRange(pos models.Position, radius int) []*PlayerWorldState {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	regionKeys := wr.getNearbyRegions(pos.X, pos.Y, radius)

	var players []*PlayerWorldState
	seen := make(map[int32]bool)

	for _, regionKey := range regionKeys {
		objectIDs, exists := wr.regions[regionKey]
		if !exists {
			continue
		}
		for _, objectID := range objectIDs {
			if seen[objectID] {
				continue
			}
			seen[objectID] = true

			state, isPlayer := wr.players[objectID]
			if !isPlayer {
				continue
			}
			if wr.calculateDistance(pos, state.Position) <= radius {
				players = append(players, state)
			}
		}
	}

	return players
}

// NPC Management

// AddNPC adds an NPC instance to the world.
func (wr *WorldRegistry) AddNPC(npc *models.NpcInstance) {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	wr.npcs[npc.ObjectID] = npc

	// Add to world objects map for spatial queries
	obj := &WorldObject{
		ID:       npc.ObjectID,
		Name:     npc.Template.Name,
		Type:     ObjectTypeNPC,
		Position: npc.Position,
	}
	wr.objects[npc.ObjectID] = obj

	// Add to spatial index
	regionKey := wr.getRegionKey(npc.Position.X, npc.Position.Y)
	wr.regions[regionKey] = append(wr.regions[regionKey], npc.ObjectID)
}

// RemoveNPC removes an NPC from the world.
func (wr *WorldRegistry) RemoveNPC(objectID int32) {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	npc, exists := wr.npcs[objectID]
	if !exists {
		return
	}

	regionKey := wr.getRegionKey(npc.Position.X, npc.Position.Y)
	wr.removeFromRegion(regionKey, objectID)

	delete(wr.npcs, objectID)
	delete(wr.objects, objectID)

	// Drop lingering targeters of this NPC (it may respawn under a new objectID).
	// (l2go-45b)
	wr.targets.dropTarget(objectID)
}

// GetNPC retrieves an NPC instance by object ID.
func (wr *WorldRegistry) GetNPC(objectID int32) (*models.NpcInstance, bool) {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	npc, exists := wr.npcs[objectID]
	return npc, exists
}

// GetNPCsInRange returns all NPC instances within range of a position.
func (wr *WorldRegistry) GetNPCsInRange(pos models.Position, radius int) []*models.NpcInstance {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	regionKeys := wr.getNearbyRegions(pos.X, pos.Y, radius)

	var result []*models.NpcInstance
	seen := make(map[int32]bool)

	for _, regionKey := range regionKeys {
		objectIDs, exists := wr.regions[regionKey]
		if !exists {
			continue
		}
		for _, objectID := range objectIDs {
			if seen[objectID] {
				continue
			}
			seen[objectID] = true

			npc, isNpc := wr.npcs[objectID]
			if !isNpc {
				continue
			}
			distance := wr.calculateDistance(pos, npc.Position)
			if distance <= radius {
				result = append(result, npc)
			}
		}
	}

	return result
}

// GetNPCCount returns the total number of NPCs in the world.
func (wr *WorldRegistry) GetNPCCount() int {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	return len(wr.npcs)
}

// GetAllNPCs returns a snapshot slice of all NPC instances in the world.
func (wr *WorldRegistry) GetAllNPCs() []*models.NpcInstance {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	result := make([]*models.NpcInstance, 0, len(wr.npcs))
	for _, npc := range wr.npcs {
		result = append(result, npc)
	}
	return result
}

// Statistics

// GetOnlinePlayerCount returns the number of online players
func (wr *WorldRegistry) GetOnlinePlayerCount() int {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	
	count := 0
	for _, state := range wr.players {
		if state.Online {
			count++
		}
	}
	
	return count
}

// GetAllPlayers returns all player states (for admin/debugging)
func (wr *WorldRegistry) GetAllPlayers() map[int32]*PlayerWorldState {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	// Return a copy to avoid race conditions
	players := make(map[int32]*PlayerWorldState)
	for k, v := range wr.players {
		players[k] = v
	}

	return players
}

// SnapshotPlayers collects all player-state pointers into buf[:0] under RLock and
// returns the (possibly regrown) slice — an allocation-free alternative to
// GetAllPlayers for the loop's per-tick/per-sweep passes, which iterate every
// player and would otherwise allocate a fresh map each call (O(N) garbage per
// sweep → GC pressure on the loop goroutine). Pass a reusable buffer to amortize
// the allocation to zero after warm-up; pass nil for a fresh slice. (l2go-3rx)
//
// Only the pointers are snapshotted, not the states, so callers see live data —
// same semantics as GetAllPlayers. Iterate the result AFTER this returns: the
// world lock is already released, so the body may safely Send or call methods that
// take the write lock (UpdatePlayerPosition, reconcile…). The states themselves
// are loop-owned, so a loop-goroutine caller reads/mutates them without further
// locking, exactly as before.
func (wr *WorldRegistry) SnapshotPlayers(buf []*PlayerWorldState) []*PlayerWorldState {
	wr.mu.RLock()
	defer wr.mu.RUnlock()

	buf = buf[:0]
	for _, state := range wr.players {
		buf = append(buf, state)
	}
	return buf
}

// Cleanup

// CleanupOfflinePlayers removes players that have been offline too long
func (wr *WorldRegistry) CleanupOfflinePlayers(ctx context.Context, maxOfflineTime time.Duration) int {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	
	now := time.Now()
	removed := 0
	
	for charID, state := range wr.players {
		if !state.Online && now.Sub(state.LastUpdate) > maxOfflineTime {
			// Remove from spatial index
			regionKey := wr.getRegionKey(state.Position.X, state.Position.Y)
			wr.removeFromRegion(regionKey, charID)

			// Remove from maps
			delete(wr.players, charID)
			delete(wr.objects, charID)

			// Drop from the reverse target index both ways (l2go-45b).
			wr.targets.set(charID, 0)
			wr.targets.dropTarget(charID)
			removed++
		}
	}
	
	if removed > 0 {
		log.Ctx(ctx).Info().
			Int("removed", removed).
			Msg("Cleaned up offline players")
	}
	
	return removed
}

// Private helper methods

// regionSize is the side length in world units of one spatial-index cell.
const regionSize = 1000

// packRegion packs region-grid coordinates into a single int64 map key: rx in the
// high 32 bits, ry in the low 32. Region coords (world coord / 1000) fit in int32
// for any realistic map, so this is collision-free and allocation-free — unlike the
// old fmt.Sprintf("%d,%d") key it replaces. (l2go-8hy)
func packRegion(rx, ry int) int64 {
	return int64(rx)<<32 | int64(int32(ry))&0xffffffff
}

// getRegionKey creates a spatial index key from world coordinates.
func (wr *WorldRegistry) getRegionKey(x, y int) int64 {
	return packRegion(x/regionSize, y/regionSize)
}

// removeFromRegion removes an object from a spatial region
func (wr *WorldRegistry) removeFromRegion(regionKey int64, objectID int32) {
	if objectIDs, exists := wr.regions[regionKey]; exists {
		for i, id := range objectIDs {
			if id == objectID {
				wr.regions[regionKey] = append(objectIDs[:i], objectIDs[i+1:]...)
				break
			}
		}
		
		// Clean up empty regions
		if len(wr.regions[regionKey]) == 0 {
			delete(wr.regions, regionKey)
		}
	}
}

// getNearbyRegions returns the packed keys of every region cell overlapping the
// radius around (x,y). Allocation-free per key (int64 packing, no string build).
func (wr *WorldRegistry) getNearbyRegions(x, y, radius int) []int64 {
	startX := (x - radius) / regionSize
	endX := (x + radius) / regionSize
	startY := (y - radius) / regionSize
	endY := (y + radius) / regionSize

	regions := make([]int64, 0, (endX-startX+1)*(endY-startY+1))
	for rx := startX; rx <= endX; rx++ {
		for ry := startY; ry <= endY; ry++ {
			regions = append(regions, packRegion(rx, ry))
		}
	}

	return regions
}

// calculateDistance calculates the 2D distance between two positions
func (wr *WorldRegistry) calculateDistance(pos1, pos2 models.Position) int {
	dx := pos1.X - pos2.X
	dy := pos1.Y - pos2.Y
	return int(math.Sqrt(float64(dx*dx + dy*dy)))
}

// UpdatePlayerRunWalkState updates a player's run/walk state
func (wr *WorldRegistry) UpdatePlayerRunWalkState(ctx context.Context, charID int32, isRunning bool) error {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	
	state, exists := wr.players[charID]
	if !exists {
		return fmt.Errorf("player not found: %d", charID)
	}
	
	// Update run/walk state
	state.IsRunning = isRunning
	state.LastUpdate = time.Now()
	
	return nil
}

// Errors
var (
	ErrPlayerNotFound = fmt.Errorf("player not found")
	ErrObjectNotFound = fmt.Errorf("object not found")
)