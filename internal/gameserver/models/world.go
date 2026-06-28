package models

import (
	"math"
	"sync"
	"time"
)

// WorldObject represents any object that exists in the game world
type WorldObject interface {
	GetObjectID() int32
	GetPosition() Position
	GetName() string
	GetObjectType() ObjectType
}

// ObjectType represents different types of world objects
type ObjectType int

const (
	ObjectTypeCharacter ObjectType = 0
	ObjectTypeNPC       ObjectType = 1
	ObjectTypeItem      ObjectType = 2
	ObjectTypeStatic    ObjectType = 3
	ObjectTypeSpawn     ObjectType = 4
	ObjectTypeDoor      ObjectType = 5
	ObjectTypeSummon    ObjectType = 6
	ObjectTypePet       ObjectType = 7
)

// World represents the game world state
type World struct {
	// Object management
	objects     map[int32]WorldObject
	regions     map[RegionKey]*WorldRegion
	objectMutex sync.RWMutex

	// Next object ID counter
	nextObjectID int32
	idMutex      sync.Mutex

	// World configuration
	config WorldConfig
}

// WorldConfig contains world configuration parameters
type WorldConfig struct {
	// Region system
	RegionWidth  int `json:"region_width"`  // Region width in game units
	RegionHeight int `json:"region_height"` // Region height in game units
	
	// Visibility system
	MaxVisibilityRange int `json:"max_visibility_range"` // Maximum visibility range
	
	// Update intervals
	ObjectUpdateInterval time.Duration `json:"object_update_interval"`
	RegionUpdateInterval time.Duration `json:"region_update_interval"`
	
	// Performance limits
	MaxObjectsPerRegion int `json:"max_objects_per_region"`
	MaxPlayersPerRegion int `json:"max_players_per_region"`
}

// NewWorld creates a new world instance
func NewWorld(config WorldConfig) *World {
	return &World{
		objects:      make(map[int32]WorldObject),
		regions:      make(map[RegionKey]*WorldRegion),
		nextObjectID: 1000000, // Start from 1M to avoid conflicts with template IDs
		config:       config,
	}
}

// GetNextObjectID generates a unique object ID
func (w *World) GetNextObjectID() int32 {
	w.idMutex.Lock()
	defer w.idMutex.Unlock()
	
	id := w.nextObjectID
	w.nextObjectID++
	return id
}

// AddObject adds an object to the world
func (w *World) AddObject(obj WorldObject) error {
	w.objectMutex.Lock()
	defer w.objectMutex.Unlock()
	
	objectID := obj.GetObjectID()
	if _, exists := w.objects[objectID]; exists {
		return ErrObjectExists
	}
	
	w.objects[objectID] = obj
	
	// Add to appropriate region
	region := w.getOrCreateRegion(obj.GetPosition())
	region.AddObject(obj)
	
	return nil
}

// RemoveObject removes an object from the world
func (w *World) RemoveObject(objectID int32) error {
	w.objectMutex.Lock()
	defer w.objectMutex.Unlock()
	
	obj, exists := w.objects[objectID]
	if !exists {
		return ErrObjectNotFound
	}
	
	delete(w.objects, objectID)
	
	// Remove from region
	region := w.getRegion(obj.GetPosition())
	if region != nil {
		region.RemoveObject(objectID)
	}
	
	return nil
}

// GetObject retrieves an object by ID
func (w *World) GetObject(objectID int32) (WorldObject, bool) {
	w.objectMutex.RLock()
	defer w.objectMutex.RUnlock()
	
	obj, exists := w.objects[objectID]
	return obj, exists
}

// MoveObject updates object position and region membership
func (w *World) MoveObject(objectID int32, newPos Position) error {
	w.objectMutex.Lock()
	defer w.objectMutex.Unlock()
	
	obj, exists := w.objects[objectID]
	if !exists {
		return ErrObjectNotFound
	}
	
	oldPos := obj.GetPosition()
	oldRegion := w.getRegion(oldPos)
	newRegion := w.getOrCreateRegion(newPos)
	
	// If moving to different region, update region membership
	if oldRegion != newRegion {
		if oldRegion != nil {
			oldRegion.RemoveObject(objectID)
		}
		newRegion.AddObject(obj)
	}
	
	return nil
}

// GetVisibleObjects returns all objects visible from a position
func (w *World) GetVisibleObjects(pos Position, range_ int) []WorldObject {
	w.objectMutex.RLock()
	defer w.objectMutex.RUnlock()
	
	var visibleObjects []WorldObject
	
	// Get all regions within visibility range
	regions := w.getRegionsInRange(pos, range_)
	
	for _, region := range regions {
		objects := region.GetObjects()
		for _, obj := range objects {
			distance := CalculateDistance(pos, obj.GetPosition())
			if distance <= float64(range_) {
				visibleObjects = append(visibleObjects, obj)
			}
		}
	}
	
	return visibleObjects
}

// GetNearbyPlayers returns all characters near a position
func (w *World) GetNearbyPlayers(pos Position, range_ int) []*Character {
	objects := w.GetVisibleObjects(pos, range_)
	var players []*Character
	
	for _, obj := range objects {
		if obj.GetObjectType() == ObjectTypeCharacter {
			if char, ok := obj.(*Character); ok {
				players = append(players, char)
			}
		}
	}
	
	return players
}

// GetObjectsInRadius returns all objects within specified radius
func (w *World) GetObjectsInRadius(pos Position, radius float64) []WorldObject {
	w.objectMutex.RLock()
	defer w.objectMutex.RUnlock()
	
	var objects []WorldObject
	regions := w.getRegionsInRange(pos, int(radius))
	
	for _, region := range regions {
		regionObjects := region.GetObjects()
		for _, obj := range regionObjects {
			distance := CalculateDistance(pos, obj.GetPosition())
			if distance <= radius {
				objects = append(objects, obj)
			}
		}
	}
	
	return objects
}

// GetWorldStats returns world statistics
func (w *World) GetWorldStats() WorldStats {
	w.objectMutex.RLock()
	defer w.objectMutex.RUnlock()
	
	stats := WorldStats{
		TotalObjects: len(w.objects),
		TotalRegions: len(w.regions),
	}
	
	// Count by object type
	for _, obj := range w.objects {
		switch obj.GetObjectType() {
		case ObjectTypeCharacter:
			stats.Characters++
		case ObjectTypeNPC:
			stats.NPCs++
		case ObjectTypeItem:
			stats.Items++
		case ObjectTypeStatic:
			stats.StaticObjects++
		case ObjectTypeSpawn:
			stats.Spawns++
		case ObjectTypeDoor:
			stats.Doors++
		case ObjectTypeSummon:
			stats.Summons++
		case ObjectTypePet:
			stats.Pets++
		}
	}
	
	return stats
}

// Region system

// RegionKey represents a region coordinate
type RegionKey struct {
	X int
	Y int
}

// WorldRegion represents a region in the world grid
type WorldRegion struct {
	Key     RegionKey              `json:"key"`
	Objects map[int32]WorldObject  `json:"-"`
	mutex   sync.RWMutex
}

// NewWorldRegion creates a new world region
func NewWorldRegion(key RegionKey) *WorldRegion {
	return &WorldRegion{
		Key:     key,
		Objects: make(map[int32]WorldObject),
	}
}

// AddObject adds an object to this region
func (r *WorldRegion) AddObject(obj WorldObject) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.Objects[obj.GetObjectID()] = obj
}

// RemoveObject removes an object from this region
func (r *WorldRegion) RemoveObject(objectID int32) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	delete(r.Objects, objectID)
}

// GetObjects returns all objects in this region
func (r *WorldRegion) GetObjects() []WorldObject {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	objects := make([]WorldObject, 0, len(r.Objects))
	for _, obj := range r.Objects {
		objects = append(objects, obj)
	}
	return objects
}

// GetObjectCount returns number of objects in this region
func (r *WorldRegion) GetObjectCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	return len(r.Objects)
}

// Helper methods for region management

func (w *World) getRegionKey(pos Position) RegionKey {
	return RegionKey{
		X: pos.X / w.config.RegionWidth,
		Y: pos.Y / w.config.RegionHeight,
	}
}

func (w *World) getRegion(pos Position) *WorldRegion {
	key := w.getRegionKey(pos)
	return w.regions[key]
}

func (w *World) getOrCreateRegion(pos Position) *WorldRegion {
	key := w.getRegionKey(pos)
	
	if region, exists := w.regions[key]; exists {
		return region
	}
	
	region := NewWorldRegion(key)
	w.regions[key] = region
	return region
}

func (w *World) getRegionsInRange(pos Position, range_ int) []*WorldRegion {
	centerKey := w.getRegionKey(pos)
	regionRange := range_ / w.config.RegionWidth + 1
	
	var regions []*WorldRegion
	
	for x := centerKey.X - regionRange; x <= centerKey.X + regionRange; x++ {
		for y := centerKey.Y - regionRange; y <= centerKey.Y + regionRange; y++ {
			key := RegionKey{X: x, Y: y}
			if region, exists := w.regions[key]; exists {
				regions = append(regions, region)
			}
		}
	}
	
	return regions
}

// Utility functions

// CalculateDistance calculates distance between two positions
func CalculateDistance(pos1, pos2 Position) float64 {
	dx := float64(pos1.X - pos2.X)
	dy := float64(pos1.Y - pos2.Y)
	dz := float64(pos1.Z - pos2.Z)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// CalculateDistance2D calculates 2D distance between two positions
func CalculateDistance2D(pos1, pos2 Position) float64 {
	dx := float64(pos1.X - pos2.X)
	dy := float64(pos1.Y - pos2.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

// CalculateSquaredDistance calculates squared distance (for performance)
func CalculateSquaredDistance(pos1, pos2 Position) float64 {
	dx := float64(pos1.X - pos2.X)
	dy := float64(pos1.Y - pos2.Y)
	dz := float64(pos1.Z - pos2.Z)
	return dx*dx + dy*dy + dz*dz
}

// IsInRange checks if two positions are within specified range
func IsInRange(pos1, pos2 Position, range_ int) bool {
	return CalculateSquaredDistance(pos1, pos2) <= float64(range_*range_)
}

// CalculateHeading calculates heading from one position to another
func CalculateHeading(from, to Position) int {
	dx := to.X - from.X
	dy := to.Y - from.Y
	
	if dx == 0 && dy == 0 {
		return 0
	}
	
	heading := int(math.Atan2(float64(dy), float64(dx)) * 180.0 / math.Pi)
	
	// Convert to L2J heading format (0-65535)
	heading = (heading + 450) % 360
	return int(float64(heading) * 65536.0 / 360.0)
}

// ValidatePosition validates if position is within world bounds
func ValidatePosition(pos Position, bounds WorldBounds) bool {
	return pos.X >= bounds.MinX && pos.X <= bounds.MaxX &&
		pos.Y >= bounds.MinY && pos.Y <= bounds.MaxY &&
		pos.Z >= bounds.MinZ && pos.Z <= bounds.MaxZ
}

// WorldBounds represents world boundaries
type WorldBounds struct {
	MinX int `json:"min_x"`
	MaxX int `json:"max_x"`
	MinY int `json:"min_y"`
	MaxY int `json:"max_y"`
	MinZ int `json:"min_z"`
	MaxZ int `json:"max_z"`
}

// WorldStats represents world statistics
type WorldStats struct {
	TotalObjects  int `json:"total_objects"`
	TotalRegions  int `json:"total_regions"`
	Characters    int `json:"characters"`
	NPCs          int `json:"npcs"`
	Items         int `json:"items"`
	StaticObjects int `json:"static_objects"`
	Spawns        int `json:"spawns"`
	Doors         int `json:"doors"`
	Summons       int `json:"summons"`
	Pets          int `json:"pets"`
}

// Spawn represents a creature spawn point
type Spawn struct {
	SpawnID    int32    `json:"spawn_id"`
	Position   Position `json:"position"`
	NPCId      int32    `json:"npc_id"`
	RespawnMin int      `json:"respawn_min"` // Minimum respawn time in seconds
	RespawnMax int      `json:"respawn_max"` // Maximum respawn time in seconds
	Count      int      `json:"count"`       // Number of NPCs to spawn
	Heading    int      `json:"heading"`     // Spawn heading
	
	// Current state
	Active     bool      `json:"active"`
	LastSpawn  time.Time `json:"last_spawn"`
	NextSpawn  time.Time `json:"next_spawn"`
}

// GetObjectID implements WorldObject interface
func (s *Spawn) GetObjectID() int32 {
	return s.SpawnID
}

// GetPosition implements WorldObject interface
func (s *Spawn) GetPosition() Position {
	return s.Position
}

// GetName implements WorldObject interface
func (s *Spawn) GetName() string {
	return "Spawn"
}

// GetObjectType implements WorldObject interface
func (s *Spawn) GetObjectType() ObjectType {
	return ObjectTypeSpawn
}

// CanSpawn returns true if spawn can respawn now
func (s *Spawn) CanSpawn() bool {
	return s.Active && time.Now().After(s.NextSpawn)
}

// SetNextSpawn sets the next spawn time
func (s *Spawn) SetNextSpawn() {
	if s.RespawnMin == s.RespawnMax {
		s.NextSpawn = time.Now().Add(time.Duration(s.RespawnMin) * time.Second)
	} else {
		// Random respawn time between min and max
		respawnTime := s.RespawnMin + int(time.Now().UnixNano()%int64(s.RespawnMax-s.RespawnMin))
		s.NextSpawn = time.Now().Add(time.Duration(respawnTime) * time.Second)
	}
	s.LastSpawn = time.Now()
}

// LocationData represents a named location in the world
type LocationData struct {
	ID       int32    `json:"id"`
	Name     string   `json:"name"`
	Position Position `json:"position"`
	Radius   int      `json:"radius"`
	Type     string   `json:"type"` // "town", "dungeon", "field", etc.
}

// TeleportLocation represents a teleportation destination
type TeleportLocation struct {
	LocationData
	Cost       int64 `json:"cost"`        // Adena cost
	MinLevel   int   `json:"min_level"`   // Minimum level required
	MaxLevel   int   `json:"max_level"`   // Maximum level allowed (0 = no limit)
	Restricted bool  `json:"restricted"`  // Requires special conditions
}

// World-related errors
var (
	ErrObjectExists        = &WorldError{"object already exists"}
	ErrObjectNotFound      = &WorldError{"object not found"}
	ErrInvalidPosition     = &WorldError{"invalid position"}
	ErrPositionOccupied    = &WorldError{"position occupied"}
	ErrOutOfBounds         = &WorldError{"position out of bounds"}
	ErrRegionFull          = &WorldError{"region full"}
	ErrInvalidObjectType   = &WorldError{"invalid object type"}
)

// WorldError represents world-related errors
type WorldError struct {
	msg string
}

func (e *WorldError) Error() string {
	return e.msg
}

// Make Character implement WorldObject interface
func (c *Character) GetObjectID() int32 {
	return c.ID
}

func (c *Character) GetPosition() Position {
	return c.Position
}

func (c *Character) GetName() string {
	return c.Name
}

func (c *Character) GetObjectType() ObjectType {
	return ObjectTypeCharacter
}