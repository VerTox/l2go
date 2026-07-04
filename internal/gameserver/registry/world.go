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

	// Spatial indexing (simple implementation)
	regions map[string][]int32             // "x,y" -> []objectIDs
}

// NewWorldRegistry creates a new world registry
func NewWorldRegistry() *WorldRegistry {
	return &WorldRegistry{
		players: make(map[int32]*PlayerWorldState),
		objects: make(map[int32]*WorldObject),
		npcs:    make(map[int32]*models.NpcInstance),
		regions: make(map[string][]int32),
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
			Str("old_region", oldRegionKey).
			Str("new_region", newRegionKey).
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

// getRegionKey creates a spatial index key from coordinates
func (wr *WorldRegistry) getRegionKey(x, y int) string {
	// Simple grid-based spatial indexing (1000x1000 units per region)
	regionX := x / 1000
	regionY := y / 1000
	return fmt.Sprintf("%d,%d", regionX, regionY)
}

// removeFromRegion removes an object from a spatial region
func (wr *WorldRegistry) removeFromRegion(regionKey string, objectID int32) {
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

// getNearbyRegions returns region keys for nearby regions
func (wr *WorldRegistry) getNearbyRegions(x, y, radius int) []string {
	regionSize := 1000
	startX := (x - radius) / regionSize
	endX := (x + radius) / regionSize
	startY := (y - radius) / regionSize
	endY := (y + radius) / regionSize
	
	var regions []string
	for rx := startX; rx <= endX; rx++ {
		for ry := startY; ry <= endY; ry++ {
			regions = append(regions, fmt.Sprintf("%d,%d", rx, ry))
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