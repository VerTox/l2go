package registry

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// Map-region tile math, per L2J MapRegionManager.getMapRegionX/Y: a world coordinate
// maps to a tile index via (pos >> 15) + offset.
const (
	mapRegionTileShift   = 15
	mapRegionOffsetX     = 20 // L2J: 9 + 11
	mapRegionOffsetY     = 18 // L2J: 10 + 8
	defaultRespawnRegion = "talking_island_town"
)

// mapRegion is one loaded region: the tiles it covers and its town respawn points.
type mapRegion struct {
	name      string
	town      string
	spawnLocs []models.Position    // regular respawn points (chaotic/other/banish excluded)
	tiles     map[[2]int]struct{}  // set of (tileX, tileY) covered by this region
}

// MapRegionRegistry maps a world position to the town respawn point of the region that
// contains it (L2J MapRegionManager, TeleportWhereType.TOWN → getSpawnLoc).
type MapRegionRegistry struct {
	mu      sync.RWMutex
	regions []*mapRegion
	def     *mapRegion // fallback region (talking_island_town)
	loaded  bool
}

// NewMapRegionRegistry creates an empty registry.
func NewMapRegionRegistry() *MapRegionRegistry { return &MapRegionRegistry{} }

// Global map-region registry instance.
var mapRegions = NewMapRegionRegistry()

// GetMapRegionRegistry returns the global map-region registry.
func GetMapRegionRegistry() *MapRegionRegistry { return mapRegions }

// XML schema for data/mapregion/*.xml (subset used for respawn).
type xmlMapRegionList struct {
	Regions []xmlMapRegion `xml:"region"`
}

type xmlMapRegion struct {
	Name     string            `xml:"name,attr"`
	Town     string            `xml:"town,attr"`
	Respawns []xmlRespawnPoint `xml:"respawnPoint"`
	Maps     []xmlMapTile      `xml:"map"`
}

type xmlRespawnPoint struct {
	X         int  `xml:"X,attr"`
	Y         int  `xml:"Y,attr"`
	Z         int  `xml:"Z,attr"`
	IsChaotic bool `xml:"isChaotic,attr"`
	IsOther   bool `xml:"isOther,attr"`
	IsBanish  bool `xml:"isBanish,attr"`
}

type xmlMapTile struct {
	X int `xml:"X,attr"`
	Y int `xml:"Y,attr"`
}

// IsLoaded reports whether map regions have been loaded.
func (r *MapRegionRegistry) IsLoaded() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded
}

// Count returns the number of loaded regions.
func (r *MapRegionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.regions)
}

// LoadFromDirectory parses every *.xml map-region file in dir.
func (r *MapRegionRegistry) LoadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read mapregion dir: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.regions = nil
	r.def = nil

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".xml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			log.Warn().Err(err).Str("file", e.Name()).Msg("mapregion: read failed")
			continue
		}
		var list xmlMapRegionList
		if err := xml.Unmarshal(data, &list); err != nil {
			log.Warn().Err(err).Str("file", e.Name()).Msg("mapregion: parse failed")
			continue
		}
		for i := range list.Regions {
			reg := buildMapRegion(&list.Regions[i])
			r.regions = append(r.regions, reg)
			if reg.name == defaultRespawnRegion {
				r.def = reg
			}
		}
	}
	r.loaded = true
	log.Info().Int("regions", len(r.regions)).Msg("Loaded map regions")
	return nil
}

func buildMapRegion(x *xmlMapRegion) *mapRegion {
	reg := &mapRegion{
		name:  x.Name,
		town:  x.Town,
		tiles: make(map[[2]int]struct{}, len(x.Maps)),
	}
	for _, rp := range x.Respawns {
		if rp.IsChaotic || rp.IsOther || rp.IsBanish {
			continue
		}
		reg.spawnLocs = append(reg.spawnLocs, models.Position{X: rp.X, Y: rp.Y, Z: rp.Z})
	}
	for _, m := range x.Maps {
		reg.tiles[[2]int{m.X, m.Y}] = struct{}{}
	}
	return reg
}

// tileIndex converts a world (x, y) to its map-region tile index.
func tileIndex(x, y int) (int, int) {
	return (x >> mapRegionTileShift) + mapRegionOffsetX, (y >> mapRegionTileShift) + mapRegionOffsetY
}

// GetRespawnPoint returns the town respawn location for the map region that contains
// (x, y), falling back to the default region (talking_island_town) when none matches.
// It returns the region's first regular spawn point, matching L2J getSpawnLoc with
// randomRespawnInTown disabled. ok is false only if no region and no default is loaded.
func (r *MapRegionRegistry) GetRespawnPoint(x, y int) (models.Position, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tx, ty := tileIndex(x, y)
	for _, reg := range r.regions {
		if len(reg.spawnLocs) == 0 {
			continue
		}
		if _, ok := reg.tiles[[2]int{tx, ty}]; ok {
			return reg.spawnLocs[0], true
		}
	}
	if r.def != nil && len(r.def.spawnLocs) > 0 {
		return r.def.spawnLocs[0], true
	}
	return models.Position{}, false
}
