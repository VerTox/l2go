package registry

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// NpcTemplateRegistry holds all NPC templates in memory.
type NpcTemplateRegistry struct {
	mu        sync.RWMutex
	templates map[int32]*models.NpcTemplate
	loaded    bool
}

// NewNpcTemplateRegistry creates a new NPC template registry.
func NewNpcTemplateRegistry() *NpcTemplateRegistry {
	return &NpcTemplateRegistry{
		templates: make(map[int32]*models.NpcTemplate),
	}
}

// Global NPC template registry instance.
var npcTemplates = NewNpcTemplateRegistry()

// GetNpcTemplateRegistry returns the global NPC template registry.
func GetNpcTemplateRegistry() *NpcTemplateRegistry {
	return npcTemplates
}

// Get returns an NPC template by ID.
func (r *NpcTemplateRegistry) Get(npcID int32) *models.NpcTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.templates[npcID]
}

// Count returns the number of loaded templates.
func (r *NpcTemplateRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.templates)
}

// IsLoaded returns true if templates have been loaded.
func (r *NpcTemplateRegistry) IsLoaded() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded
}

// LoadFromDirectory loads all NPC templates from XML files in the given directory.
func (r *NpcTemplateRegistry) LoadFromDirectory(dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Info().Str("dir", dir).Msg("Loading NPC templates from directory")

	files, err := filepath.Glob(filepath.Join(dir, "*.xml"))
	if err != nil {
		return fmt.Errorf("failed to list XML files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no XML files found in %s", dir)
	}

	totalNPCs := 0
	for _, file := range files {
		count, err := r.loadXMLFile(file)
		if err != nil {
			log.Warn().Err(err).Str("file", file).Msg("Failed to load NPC file")
			continue
		}
		totalNPCs += count
	}

	r.loaded = true
	log.Info().Int("total", totalNPCs).Int("files", len(files)).Msg("NPC templates loaded")
	return nil
}

// ---- XML structures for L2J NPC data ----

type xmlNpcList struct {
	XMLName xml.Name `xml:"list"`
	NPCs    []xmlNpc `xml:"npc"`
}

type xmlNpc struct {
	ID    int32  `xml:"id,attr"`
	Level int    `xml:"level,attr"`
	Type  string `xml:"type,attr"`
	Name  string `xml:"name,attr"`

	Race      string        `xml:"race"`
	Sex       string        `xml:"sex"`
	Equipment *xmlEquipment `xml:"equipment"`
	Stats     *xmlStats     `xml:"stats"`
	AI        *xmlAI        `xml:"ai"`
	Collision *xmlCollision `xml:"collision"`
	Status    *xmlStatus    `xml:"status"`
}

type xmlEquipment struct {
	RHand int32 `xml:"rhand,attr"`
	LHand int32 `xml:"lhand,attr"`
	Chest int32 `xml:"chest,attr"`
}

type xmlStats struct {
	STR int `xml:"str,attr"`
	INT int `xml:"int,attr"`
	DEX int `xml:"dex,attr"`
	WIT int `xml:"wit,attr"`
	CON int `xml:"con,attr"`
	MEN int `xml:"men,attr"`

	Vitals  *xmlVitals  `xml:"vitals"`
	Attack  *xmlAttack  `xml:"attack"`
	Defence *xmlDefence `xml:"defence"`
	Speed   *xmlSpeed   `xml:"speed"`
}

type xmlVitals struct {
	HP string `xml:"hp,attr"`
	MP string `xml:"mp,attr"`
}

type xmlAttack struct {
	Physical    string `xml:"physical,attr"`
	Magical     string `xml:"magical,attr"`
	AttackSpeed string `xml:"attackSpeed,attr"`
	Range       string `xml:"range,attr"`
	Critical    string `xml:"critical,attr"`
}

type xmlDefence struct {
	Physical string `xml:"physical,attr"`
	Magical  string `xml:"magical,attr"`
}

type xmlSpeed struct {
	Walk *xmlSpeedEntry `xml:"walk"`
	Run  *xmlSpeedEntry `xml:"run"`
}

type xmlSpeedEntry struct {
	Ground string `xml:"ground,attr"`
}

type xmlCollision struct {
	Radius *xmlCollisionValue `xml:"radius"`
	Height *xmlCollisionValue `xml:"height"`
}

type xmlCollisionValue struct {
	Normal string `xml:"normal,attr"`
}

type xmlAI struct {
	AggroRange   string `xml:"aggroRange,attr"`
	IsAggressive string `xml:"isAggressive,attr"`
}

type xmlStatus struct {
	Undying string `xml:"undying,attr"`
}

// loadXMLFile loads NPCs from a single XML file.
func (r *NpcTemplateRegistry) loadXMLFile(filename string) (int, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	var list xmlNpcList
	if err := xml.Unmarshal(data, &list); err != nil {
		return 0, fmt.Errorf("failed to parse XML: %w", err)
	}

	count := 0
	for _, xn := range list.NPCs {
		tpl := convertXMLNpc(xn)
		r.templates[tpl.ID] = tpl
		count++
	}

	return count, nil
}

func convertXMLNpc(xn xmlNpc) *models.NpcTemplate {
	t := &models.NpcTemplate{
		ID:        xn.ID,
		DisplayID: xn.ID,
		Name:      xn.Name,
		Level:     xn.Level,
		Type:      xn.Type,
		Race:      xn.Race,
		Sex:       xn.Sex,

		// Defaults
		Attackable: true,
		Targetable: true,
		ShowName:   true,
		CanMove:    true,
	}

	// Type-based defaults
	switch xn.Type {
	case "L2Npc", "L2Merchant", "L2Teleporter", "L2Warehouse":
		t.Attackable = false
	}

	// Equipment
	if xn.Equipment != nil {
		t.RHand = xn.Equipment.RHand
		t.LHand = xn.Equipment.LHand
		t.Chest = xn.Equipment.Chest
	}

	// Stats
	if xn.Stats != nil {
		if xn.Stats.Vitals != nil {
			t.HP = parseFloat64(xn.Stats.Vitals.HP)
			t.MP = parseFloat64(xn.Stats.Vitals.MP)
		}
		if xn.Stats.Attack != nil {
			t.PAtk = parseFloat64(xn.Stats.Attack.Physical)
			t.MAtk = parseFloat64(xn.Stats.Attack.Magical)
			t.PAtkSpd = parseIntSafe(xn.Stats.Attack.AttackSpeed)
			t.AttackRange = parseIntSafe(xn.Stats.Attack.Range)
			t.CritRate = parseIntSafe(xn.Stats.Attack.Critical)
		}
		if xn.Stats.Defence != nil {
			t.PDef = parseFloat64(xn.Stats.Defence.Physical)
			t.MDef = parseFloat64(xn.Stats.Defence.Magical)
		}
		if xn.Stats.Speed != nil {
			if xn.Stats.Speed.Walk != nil {
				t.WalkSpd = parseIntSafe(xn.Stats.Speed.Walk.Ground)
			}
			if xn.Stats.Speed.Run != nil {
				t.RunSpd = parseIntSafe(xn.Stats.Speed.Run.Ground)
			}
		}
	}

	// Collision
	if xn.Collision != nil {
		if xn.Collision.Radius != nil {
			t.CollisionRadius = parseFloat64(xn.Collision.Radius.Normal)
		}
		if xn.Collision.Height != nil {
			t.CollisionHeight = parseFloat64(xn.Collision.Height.Normal)
		}
	}

	// AI
	if xn.AI != nil {
		t.AggroRange = parseIntSafe(xn.AI.AggroRange)
	}

	return t
}

func parseFloat64(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func parseIntSafe(s string) int {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return int(f)
}
