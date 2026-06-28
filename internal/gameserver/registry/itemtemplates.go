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
)

// ItemType2 represents item type for packets (Type2 field in L2J)
// Used in ItemList packet for proper client rendering
type ItemType2 int32

const (
	ItemType2Weapon    ItemType2 = 0 // Weapon
	ItemType2Armor     ItemType2 = 1 // Armor/Shield
	ItemType2Accessory ItemType2 = 2 // Jewelry (rings, earrings, necklace)
	ItemType2Quest     ItemType2 = 3 // Quest items
	ItemType2Money     ItemType2 = 4 // Adena/currency
	ItemType2Other     ItemType2 = 5 // Everything else (consumables, materials, etc.)
)

// ItemGrade represents item grade (crystal type in L2J)
type ItemGrade int32

const (
	GradeNone ItemGrade = 0 // No-grade
	GradeD    ItemGrade = 1
	GradeC    ItemGrade = 2
	GradeB    ItemGrade = 3
	GradeA    ItemGrade = 4
	GradeS    ItemGrade = 5
	GradeS80  ItemGrade = 6
	GradeS84  ItemGrade = 7
)

// WeaponType represents weapon type
type WeaponType string

const (
	WeaponSword       WeaponType = "SWORD"
	WeaponBlunt       WeaponType = "BLUNT"
	WeaponDagger      WeaponType = "DAGGER"
	WeaponBow         WeaponType = "BOW"
	WeaponPole        WeaponType = "POLE"
	WeaponFist        WeaponType = "FIST"
	WeaponDualSword   WeaponType = "DUAL"
	WeaponEtc         WeaponType = "ETC"
	WeaponAncientSword WeaponType = "ANCIENTSWORD"
	WeaponCrossbow    WeaponType = "CROSSBOW"
	WeaponRapier      WeaponType = "RAPIER"
	WeaponDualDagger  WeaponType = "DUALDAGGER"
	WeaponBigSword    WeaponType = "BIGSWORD"
	WeaponBigBlunt    WeaponType = "BIGBLUNT"
	WeaponDualBlunt   WeaponType = "DUALBLUNT"
	WeaponNone        WeaponType = "NONE"
)

// ArmorType represents armor type
type ArmorType string

const (
	ArmorLight  ArmorType = "LIGHT"
	ArmorHeavy  ArmorType = "HEAVY"
	ArmorMagic  ArmorType = "MAGIC"
	ArmorSigil  ArmorType = "SIGIL"
	ArmorNone   ArmorType = "NONE"
)

// EtcItemType represents EtcItem subtypes
type EtcItemType string

const (
	EtcArrow     EtcItemType = "ARROW"
	EtcBolt      EtcItemType = "BOLT"
	EtcPotion    EtcItemType = "POTION"
	EtcScroll    EtcItemType = "SCROLL"
	EtcRecipe    EtcItemType = "RECIPE"
	EtcMaterial  EtcItemType = "MATERIAL"
	EtcLure      EtcItemType = "LURE"
	EtcPetCollar EtcItemType = "PET_COLLAR"
	EtcOther     EtcItemType = "OTHER"
)

// ItemTemplate represents complete item template data from L2J XML
type ItemTemplate struct {
	// Primary identification
	ID   int32  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "Weapon", "Armor", "EtcItem"

	// Body part and slot
	BodyPart     string `json:"body_part"`      // XML value: "rhand", "lhand", "chest", etc.
	BodyPartCode int32  `json:"body_part_code"` // Computed bitmask for packets

	// Weapon properties
	WeaponType   WeaponType `json:"weapon_type"`
	IsMagicWeapon bool      `json:"is_magic_weapon"`

	// Armor properties
	ArmorType ArmorType `json:"armor_type"`

	// EtcItem properties
	EtcItemType EtcItemType `json:"etcitem_type"`

	// Basic stats
	Weight       int   `json:"weight"`
	Price        int64 `json:"price"`
	CrystalType  ItemGrade `json:"crystal_type"`  // Grade
	CrystalCount int   `json:"crystal_count"`

	// Combat stats (from <for> section)
	PAtk     int `json:"p_atk"`
	MAtk     int `json:"m_atk"`
	PDef     int `json:"p_def"`
	MDef     int `json:"m_def"`
	PAtkSpd  int `json:"p_atk_spd"`
	MAtkSpd  int `json:"m_atk_spd"`
	CritRate int `json:"crit_rate"`

	// Item properties
	Stackable   bool `json:"stackable"`
	Tradeable   bool `json:"tradeable"`
	Droppable   bool `json:"droppable"`
	Sellable    bool `json:"sellable"`
	Depositable bool `json:"depositable"`
	Enchantable bool `json:"enchantable"`
	Premium     bool `json:"premium"`

	// Weapon-specific
	Soulshots   int `json:"soulshots"`
	Spiritshots int `json:"spiritshots"`
	AttackRange int `json:"attack_range"`
	RandomDamage int `json:"random_damage"`

	// Icon
	Icon string `json:"icon"`

	// Computed fields
	Type2 ItemType2 `json:"type2"` // Computed type for packets
}

// ItemTemplateRegistry holds all item templates in memory
type ItemTemplateRegistry struct {
	mu        sync.RWMutex
	templates map[int32]*ItemTemplate
	loaded    bool
}

// NewItemTemplateRegistry creates a new item template registry
func NewItemTemplateRegistry() *ItemTemplateRegistry {
	return &ItemTemplateRegistry{
		templates: make(map[int32]*ItemTemplate),
	}
}

// Global item template registry instance
var itemTemplates = NewItemTemplateRegistry()

// GetItemTemplateRegistry returns the global item template registry
func GetItemTemplateRegistry() *ItemTemplateRegistry {
	return itemTemplates
}

// Get returns item template by ID
func (r *ItemTemplateRegistry) Get(itemID int32) *ItemTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.templates[itemID]
}

// Count returns number of loaded templates
func (r *ItemTemplateRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.templates)
}

// IsLoaded returns true if templates are loaded
func (r *ItemTemplateRegistry) IsLoaded() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded
}

// LoadFromDirectory loads all item templates from XML files in directory
func (r *ItemTemplateRegistry) LoadFromDirectory(dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Info().Str("dir", dir).Msg("Loading item templates from directory")

	files, err := filepath.Glob(filepath.Join(dir, "*.xml"))
	if err != nil {
		return fmt.Errorf("failed to list XML files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no XML files found in %s", dir)
	}

	totalItems := 0
	for _, file := range files {
		count, err := r.loadXMLFile(file)
		if err != nil {
			log.Warn().Err(err).Str("file", file).Msg("Failed to load item file")
			continue
		}
		totalItems += count
	}

	r.loaded = true
	log.Info().Int("total", totalItems).Int("files", len(files)).Msg("Item templates loaded")
	return nil
}

// XML structures for parsing L2J item data

type xmlItemList struct {
	XMLName xml.Name  `xml:"list"`
	Items   []xmlItem `xml:"item"`
}

type xmlItem struct {
	ID   int32  `xml:"id,attr"`
	Type string `xml:"type,attr"`
	Name string `xml:"name,attr"`
	Sets []xmlSet `xml:"set"`
	For  *xmlFor  `xml:"for"`
}

type xmlSet struct {
	Name string `xml:"name,attr"`
	Val  string `xml:"val,attr"`
}

type xmlFor struct {
	Sets    []xmlForSet `xml:"set"`
	Adds    []xmlForAdd `xml:"add"`
}

type xmlForSet struct {
	Stat string `xml:"stat,attr"`
	Val  string `xml:"val,attr"`
}

type xmlForAdd struct {
	Stat string `xml:"stat,attr"`
	Val  string `xml:"val,attr"`
}

// loadXMLFile loads items from a single XML file
func (r *ItemTemplateRegistry) loadXMLFile(filename string) (int, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	var list xmlItemList
	if err := xml.Unmarshal(data, &list); err != nil {
		return 0, fmt.Errorf("failed to parse XML: %w", err)
	}

	count := 0
	for _, xmlItem := range list.Items {
		template := r.convertXMLItem(xmlItem)
		r.templates[template.ID] = template
		count++
	}

	return count, nil
}

// convertXMLItem converts XML item to ItemTemplate
func (r *ItemTemplateRegistry) convertXMLItem(xi xmlItem) *ItemTemplate {
	t := &ItemTemplate{
		ID:   xi.ID,
		Name: xi.Name,
		Type: xi.Type,

		// Defaults
		Stackable:   false,
		Tradeable:   true,
		Droppable:   true,
		Sellable:    true,
		Depositable: true,
		Enchantable: false,
		Premium:     false,
	}

	// Parse set attributes
	for _, set := range xi.Sets {
		r.applySetAttribute(t, set.Name, set.Val)
	}

	// Parse combat stats from <for> section
	if xi.For != nil {
		for _, stat := range xi.For.Sets {
			r.applyStatAttribute(t, stat.Stat, stat.Val)
		}
		for _, add := range xi.For.Adds {
			r.applyStatAttribute(t, add.Stat, add.Val)
		}
	}

	// Compute body part code
	t.BodyPartCode = bodyPartToCode(t.BodyPart)

	// Compute Type2 for packets
	t.Type2 = r.computeType2(t)

	return t
}

// applySetAttribute applies a set attribute to template
func (r *ItemTemplateRegistry) applySetAttribute(t *ItemTemplate, name, val string) {
	switch name {
	case "bodypart":
		t.BodyPart = val
	case "weapon_type":
		t.WeaponType = WeaponType(strings.ToUpper(val))
	case "armor_type":
		t.ArmorType = ArmorType(strings.ToUpper(val))
	case "etcitem_type":
		t.EtcItemType = EtcItemType(strings.ToUpper(val))
	case "weight":
		t.Weight = parseInt(val)
	case "price":
		t.Price = parseInt64(val)
	case "crystal_type":
		t.CrystalType = parseCrystalType(val)
	case "crystal_count":
		t.CrystalCount = parseInt(val)
	case "soulshots":
		t.Soulshots = parseInt(val)
	case "spiritshots":
		t.Spiritshots = parseInt(val)
	case "attack_range":
		t.AttackRange = parseInt(val)
	case "random_damage":
		t.RandomDamage = parseInt(val)
	case "icon":
		t.Icon = val
	case "is_stackable":
		t.Stackable = parseBool(val)
	case "is_tradable":
		t.Tradeable = parseBool(val)
	case "is_droppable":
		t.Droppable = parseBool(val)
	case "is_sellable":
		t.Sellable = parseBool(val)
	case "is_depositable":
		t.Depositable = parseBool(val)
	case "enchant_enabled":
		t.Enchantable = parseBool(val) || val == "1"
	case "is_premium":
		t.Premium = parseBool(val)
	case "is_magic_weapon":
		t.IsMagicWeapon = parseBool(val)
	}
}

// applyStatAttribute applies a stat attribute from <for> section
func (r *ItemTemplateRegistry) applyStatAttribute(t *ItemTemplate, stat, val string) {
	intVal := parseInt(val)
	switch stat {
	case "pAtk":
		t.PAtk = intVal
	case "mAtk":
		t.MAtk = intVal
	case "pDef":
		t.PDef = intVal
	case "mDef":
		t.MDef = intVal
	case "pAtkSpd":
		t.PAtkSpd = intVal
	case "mAtkSpd":
		t.MAtkSpd = intVal
	case "critRate":
		t.CritRate = intVal
	}
}

// computeType2 computes the Type2 value for packets based on item properties
func (r *ItemTemplateRegistry) computeType2(t *ItemTemplate) ItemType2 {
	switch t.Type {
	case "Weapon":
		return ItemType2Weapon
	case "Armor":
		// Check if it's armor or accessory based on body part
		switch t.BodyPart {
		case "neck", "rear", "lear", "rfinger", "lfinger", "rbracelet", "lbracelet":
			return ItemType2Accessory
		default:
			return ItemType2Armor
		}
	case "EtcItem":
		// Special case for Adena
		if t.ID == 57 {
			return ItemType2Money
		}
		// Check if it's a quest item (we'd need more data, for now use Other)
		return ItemType2Other
	default:
		return ItemType2Other
	}
}

// bodyPartToCode converts body part string to bitmask code
func bodyPartToCode(bodyPart string) int32 {
	switch strings.ToLower(bodyPart) {
	case "underwear":
		return 0x0001 // SLOT_UNDERWEAR
	case "rear":
		return 0x0002 // SLOT_R_EAR
	case "lear":
		return 0x0004 // SLOT_L_EAR
	case "neck":
		return 0x0008 // SLOT_NECK
	case "rfinger":
		return 0x0010 // SLOT_R_FINGER
	case "lfinger":
		return 0x0020 // SLOT_L_FINGER
	case "head":
		return 0x0040 // SLOT_HEAD
	case "rhand":
		return 0x0080 // SLOT_R_HAND
	case "lhand":
		return 0x0100 // SLOT_L_HAND
	case "gloves":
		return 0x0200 // SLOT_GLOVES
	case "chest":
		return 0x0400 // SLOT_CHEST
	case "legs":
		return 0x0800 // SLOT_LEGS
	case "feet":
		return 0x1000 // SLOT_FEET
	case "back", "cloak":
		return 0x2000 // SLOT_BACK (cloak)
	case "lrhand":
		return 0x4000 // SLOT_LR_HAND (two-handed)
	case "onepiece", "fullarmor":
		return 0x8000 // SLOT_FULL_ARMOR (chest+legs combined)
	case "hair":
		return 0x010000 // SLOT_HAIR
	case "alldress":
		return 0x020000 // SLOT_ALLDRESS
	case "hair2", "dhair":
		return 0x040000 // SLOT_HAIR2
	case "hairall":
		return 0x050000 // SLOT_HAIRALL (hair + hair2)
	case "rbracelet":
		return 0x100000 // SLOT_R_BRACELET
	case "lbracelet":
		return 0x200000 // SLOT_L_BRACELET
	case "deco", "deco1":
		return 0x400000 // SLOT_DECO
	case "belt":
		return 0x10000000 // SLOT_BELT
	case "wolf", "hatchling", "strider", "babypet", "greatwolf":
		return 0x20000000 // Pet equipment
	default:
		return 0
	}
}

// parseCrystalType parses crystal type string to ItemGrade
func parseCrystalType(val string) ItemGrade {
	switch strings.ToUpper(val) {
	case "D":
		return GradeD
	case "C":
		return GradeC
	case "B":
		return GradeB
	case "A":
		return GradeA
	case "S":
		return GradeS
	case "S80":
		return GradeS80
	case "S84":
		return GradeS84
	default:
		return GradeNone
	}
}

// parseInt safely parses int from string
func parseInt(val string) int {
	i, _ := strconv.Atoi(val)
	return i
}

// parseInt64 safely parses int64 from string
func parseInt64(val string) int64 {
	i, _ := strconv.ParseInt(val, 10, 64)
	return i
}

// parseBool parses boolean from string
func parseBool(val string) bool {
	return strings.ToLower(val) == "true" || val == "1"
}

// GetBodyPartCode returns body part bitmask for an item
// This should be used when building ItemList packets
func GetBodyPartCode(itemID int32) int32 {
	template := itemTemplates.Get(itemID)
	if template != nil {
		return template.BodyPartCode
	}
	return 0
}

// GetItemType2 returns Type2 value for an item (for packets)
func GetItemType2(itemID int32) ItemType2 {
	template := itemTemplates.Get(itemID)
	if template != nil {
		return template.Type2
	}
	return ItemType2Other
}

// IsStackable returns true if item is stackable
func IsItemStackable(itemID int32) bool {
	template := itemTemplates.Get(itemID)
	if template != nil {
		return template.Stackable
	}
	// Default: low ID items (under 1000) are often equipment, others may be stackable
	return itemID >= 1000
}

// GetItemName returns item name
func GetItemName(itemID int32) string {
	template := itemTemplates.Get(itemID)
	if template != nil {
		return template.Name
	}
	return fmt.Sprintf("Item #%d", itemID)
}

// GetItemWeight returns item weight
func GetItemWeight(itemID int32) int {
	template := itemTemplates.Get(itemID)
	if template != nil {
		return template.Weight
	}
	return 0
}
