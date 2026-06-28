package models

import (
	"time"
)

// CharacterItem represents an item instance owned by a character
type CharacterItem struct {
	// Primary identification
	ObjectID int32  `json:"object_id" db:"object_id"`
	OwnerID  int32  `json:"owner_id" db:"owner_id"`
	ItemID   int32  `json:"item_id" db:"item_id"`
	Name     string `json:"name" db:"name"`
	Icon     string `json:"icon" db:"icon"`
	Weight   int    `json:"weight" db:"weight"`

	// Quantity and location
	Count   int64  `json:"count" db:"count"`
	Loc     string `json:"loc" db:"loc"`
	LocData int    `json:"loc_data" db:"loc_data"`

	// Enhancement system
	EnchantLevel int `json:"enchant_level" db:"enchant_level"`

	// Metadata
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// L2J additional fields
	CustomType1 int `json:"custom_type1" db:"custom_type1"`
	CustomType2 int `json:"custom_type2" db:"custom_type2"`
	ManaLeft    int `json:"mana_left" db:"mana_left"`
	Time        int `json:"time" db:"time"`

	// Augmentation system
	AugmentationID     int `json:"augmentation_id" db:"augmentation_id"`
	AugmentationSkill1 int `json:"augmentation_skill1" db:"augmentation_skill1"`
	AugmentationSkill2 int `json:"augmentation_skill2" db:"augmentation_skill2"`

	// Elemental attributes
	AttributeFire  int `json:"attribute_fire" db:"attribute_fire"`
	AttributeWater int `json:"attribute_water" db:"attribute_water"`
	AttributeWind  int `json:"attribute_wind" db:"attribute_wind"`
	AttributeEarth int `json:"attribute_earth" db:"attribute_earth"`
	AttributeHoly  int `json:"attribute_holy" db:"attribute_holy"`
	AttributeDark  int `json:"attribute_dark" db:"attribute_dark"`

	// Visual and special properties
	VisualID    int  `json:"visual_id" db:"visual_id"`
	IsBlessed   bool `json:"is_blessed" db:"is_blessed"`
	IsProtected bool `json:"is_protected" db:"is_protected"`
}

// ItemLocation represents item storage locations
type ItemLocation string

const (
	LocInventory ItemLocation = "INVENTORY"
	LocPaperdoll ItemLocation = "PAPERDOLL"
	LocWarehouse ItemLocation = "WAREHOUSE"
	LocClanWH    ItemLocation = "CLAN_WH"
	LocPet       ItemLocation = "PET"
	LocPetEquip  ItemLocation = "PET_EQUIP"
	LocFreight   ItemLocation = "FREIGHT"
)

// PaperdollSlot represents equipment slots (matches Java L2J Inventory.PAPERDOLL_* constants)
type PaperdollSlot int

const (
	SlotUnder     PaperdollSlot = 0  // PAPERDOLL_UNDER - underwear
	SlotHead      PaperdollSlot = 1  // PAPERDOLL_HEAD - helmet
	SlotHair      PaperdollSlot = 2  // PAPERDOLL_HAIR - hair accessory
	SlotHair2     PaperdollSlot = 3  // PAPERDOLL_HAIR2 - hair accessory 2
	SlotNeck      PaperdollSlot = 4  // PAPERDOLL_NECK - necklace
	SlotRHand     PaperdollSlot = 5  // PAPERDOLL_RHAND - right hand weapon
	SlotChest     PaperdollSlot = 6  // PAPERDOLL_CHEST - chest armor
	SlotLHand     PaperdollSlot = 7  // PAPERDOLL_LHAND - left hand weapon/shield
	SlotREar      PaperdollSlot = 8  // PAPERDOLL_REAR - right earring
	SlotLEar      PaperdollSlot = 9  // PAPERDOLL_LEAR - left earring
	SlotGloves    PaperdollSlot = 10 // PAPERDOLL_GLOVES - gloves
	SlotLegs      PaperdollSlot = 11 // PAPERDOLL_LEGS - leg armor
	SlotFeet      PaperdollSlot = 12 // PAPERDOLL_FEET - boots
	SlotRFinger   PaperdollSlot = 13 // PAPERDOLL_RFINGER - right ring
	SlotLFinger   PaperdollSlot = 14 // PAPERDOLL_LFINGER - left ring
	SlotLBracelet PaperdollSlot = 15 // PAPERDOLL_LBRACELET - left bracelet
	SlotRBracelet PaperdollSlot = 16 // PAPERDOLL_RBRACELET - right bracelet
	SlotDeco1     PaperdollSlot = 17 // PAPERDOLL_DECO1 - decoration 1
	SlotDeco2     PaperdollSlot = 18 // PAPERDOLL_DECO2 - decoration 2
	SlotDeco3     PaperdollSlot = 19 // PAPERDOLL_DECO3 - decoration 3
	SlotDeco4     PaperdollSlot = 20 // PAPERDOLL_DECO4 - decoration 4
	SlotDeco5     PaperdollSlot = 21 // PAPERDOLL_DECO5 - decoration 5
	SlotDeco6     PaperdollSlot = 22 // PAPERDOLL_DECO6 - decoration 6
	SlotBack      PaperdollSlot = 23 // PAPERDOLL_CLOAK - cloak/back
	SlotBelt      PaperdollSlot = 24 // PAPERDOLL_BELT - belt
	SlotLRHand    PaperdollSlot = 25 // Two-handed weapon (virtual slot for L2Go)
)

// PaperdollSlotCodes maps paperdoll slots to body part bitmasks (L2Item.SLOT_* constants)
// These are the bitmasks used in ItemList/UserInfo packets for BodyPart field
var PaperdollSlotCodes = map[PaperdollSlot]int32{
	SlotUnder:     0x0001,     // SLOT_UNDERWEAR
	SlotHead:      0x0040,     // SLOT_HEAD
	SlotHair:      0x010000,   // SLOT_HAIR
	SlotHair2:     0x040000,   // SLOT_HAIR2
	SlotNeck:      0x0008,     // SLOT_NECK
	SlotRHand:     0x0080,     // SLOT_R_HAND
	SlotChest:     0x0400,     // SLOT_CHEST
	SlotLHand:     0x0100,     // SLOT_L_HAND
	SlotREar:      0x0002,     // SLOT_R_EAR
	SlotLEar:      0x0004,     // SLOT_L_EAR
	SlotGloves:    0x0200,     // SLOT_GLOVES
	SlotLegs:      0x0800,     // SLOT_LEGS
	SlotFeet:      0x1000,     // SLOT_FEET
	SlotRFinger:   0x0010,     // SLOT_R_FINGER
	SlotLFinger:   0x0020,     // SLOT_L_FINGER
	SlotLBracelet: 0x200000,   // SLOT_L_BRACELET
	SlotRBracelet: 0x100000,   // SLOT_R_BRACELET
	SlotDeco1:     0x400000,   // SLOT_DECO (shared for all deco slots)
	SlotDeco2:     0x400000,   // SLOT_DECO
	SlotDeco3:     0x400000,   // SLOT_DECO
	SlotDeco4:     0x400000,   // SLOT_DECO
	SlotDeco5:     0x400000,   // SLOT_DECO
	SlotDeco6:     0x400000,   // SLOT_DECO
	SlotBack:      0x2000,     // SLOT_BACK (cloak)
	SlotBelt:      0x10000000, // SLOT_BELT
	SlotLRHand:    0x4000,     // SLOT_LR_HAND (two-handed)
}

// Body part bitmask constants for dual-slot items
const (
	BodyPartLREar    int32 = 0x0006     // Both ears (0x0002 | 0x0004)
	BodyPartLRFinger int32 = 0x0030     // Both fingers (0x0010 | 0x0020)
	BodyPartLRHand   int32 = 0x4000     // Two-handed weapon
	BodyPartFullArmor int32 = 0x8000    // Full armor (chest+legs)
	BodyPartHairAll  int32 = 0x050000   // Both hair slots (0x010000 | 0x040000)
)

// BodyPartToPaperdollSlot maps a body part bitmask to the primary paperdoll slot.
// For dual-slot items (ear, finger), returns the "right" slot; caller handles fallback to "left".
func BodyPartToPaperdollSlot(bodyPartCode int32) (PaperdollSlot, bool) {
	switch bodyPartCode {
	case 0x0001: // SLOT_UNDERWEAR
		return SlotUnder, true
	case 0x0002: // SLOT_R_EAR
		return SlotREar, true
	case 0x0004: // SLOT_L_EAR
		return SlotLEar, true
	case 0x0006: // SLOT_LR_EAR — both ears
		return SlotREar, true
	case 0x0008: // SLOT_NECK
		return SlotNeck, true
	case 0x0010: // SLOT_R_FINGER
		return SlotRFinger, true
	case 0x0020: // SLOT_L_FINGER
		return SlotLFinger, true
	case 0x0030: // SLOT_LR_FINGER — both fingers
		return SlotRFinger, true
	case 0x0040: // SLOT_HEAD
		return SlotHead, true
	case 0x0080: // SLOT_R_HAND
		return SlotRHand, true
	case 0x0100: // SLOT_L_HAND
		return SlotLHand, true
	case 0x0200: // SLOT_GLOVES
		return SlotGloves, true
	case 0x0400: // SLOT_CHEST
		return SlotChest, true
	case 0x0800: // SLOT_LEGS
		return SlotLegs, true
	case 0x1000: // SLOT_FEET
		return SlotFeet, true
	case 0x2000: // SLOT_BACK (cloak)
		return SlotBack, true
	case 0x4000: // SLOT_LR_HAND — two-handed weapon → right hand
		return SlotRHand, true
	case 0x8000: // SLOT_FULL_ARMOR → chest slot
		return SlotChest, true
	case 0x010000: // SLOT_HAIR
		return SlotHair, true
	case 0x040000: // SLOT_HAIR2
		return SlotHair2, true
	case 0x050000: // SLOT_HAIRALL → hair slot
		return SlotHair, true
	case 0x100000: // SLOT_R_BRACELET
		return SlotRBracelet, true
	case 0x200000: // SLOT_L_BRACELET
		return SlotLBracelet, true
	case 0x400000: // SLOT_DECO
		return SlotDeco1, true
	case 0x10000000: // SLOT_BELT
		return SlotBelt, true
	default:
		return 0, false
	}
}

// IsDualSlot returns true if the body part bitmask represents a dual-slot item
// (earrings, rings, hair accessories that can go in either slot)
func IsDualSlot(bodyPartCode int32) bool {
	switch bodyPartCode {
	case BodyPartLREar, BodyPartLRFinger, BodyPartHairAll:
		return true
	default:
		return false
	}
}

// IsTwoHanded returns true if the body part bitmask is for a two-handed weapon
func IsTwoHanded(bodyPartCode int32) bool {
	return bodyPartCode == BodyPartLRHand
}

// IsFullArmor returns true if the body part bitmask is for full armor (chest+legs)
func IsFullArmor(bodyPartCode int32) bool {
	return bodyPartCode == BodyPartFullArmor
}

// ElementalAttribute represents elemental attribute types
type ElementalAttribute int

const (
	AttributeNone  ElementalAttribute = -1
	AttributeFIRE  ElementalAttribute = 0
	AttributeWATER ElementalAttribute = 1
	AttributeWIND  ElementalAttribute = 2
	AttributeEARTH ElementalAttribute = 3
	AttributeHOLY  ElementalAttribute = 4
	AttributeDARK  ElementalAttribute = 5
)

// IsStackable returns true if item can be stacked
func (i *CharacterItem) IsStackable() bool {
	// Items with count > 1 are typically stackable
	// This would normally check item template data
	return i.Count > 1
}

// IsEquipped returns true if item is currently equipped
func (i *CharacterItem) IsEquipped() bool {
	return i.Loc == string(LocPaperdoll) && i.LocData >= 0
}

// IsInInventory returns true if item is in character inventory
func (i *CharacterItem) IsInInventory() bool {
	return i.Loc == string(LocInventory)
}

// IsInWarehouse returns true if item is in any warehouse
func (i *CharacterItem) IsInWarehouse() bool {
	return i.Loc == string(LocWarehouse) || i.Loc == string(LocClanWH) || i.Loc == string(LocFreight)
}

// IsEnchanted returns true if item has enchantment
func (i *CharacterItem) IsEnchanted() bool {
	return i.EnchantLevel > 0
}

// IsAugmented returns true if item has augmentation
func (i *CharacterItem) IsAugmented() bool {
	return i.AugmentationID > 0
}

// HasElementalAttribute returns true if item has any elemental attribute
func (i *CharacterItem) HasElementalAttribute() bool {
	return i.AttributeFire > 0 || i.AttributeWater > 0 || i.AttributeWind > 0 ||
		i.AttributeEarth > 0 || i.AttributeHoly > 0 || i.AttributeDark > 0
}

// GetStrongestElementalAttribute returns the strongest elemental attribute
func (i *CharacterItem) GetStrongestElementalAttribute() (ElementalAttribute, int) {
	maxValue := 0
	maxAttr := AttributeNone

	attributes := []struct {
		attr  ElementalAttribute
		value int
	}{
		{AttributeFIRE, i.AttributeFire},
		{AttributeWATER, i.AttributeWater},
		{AttributeWIND, i.AttributeWind},
		{AttributeEARTH, i.AttributeEarth},
		{AttributeHOLY, i.AttributeHoly},
		{AttributeDARK, i.AttributeDark},
	}

	for _, attr := range attributes {
		if attr.value > maxValue {
			maxValue = attr.value
			maxAttr = attr.attr
		}
	}

	return maxAttr, maxValue
}

// IsTemporary returns true if item has expiration time
func (i *CharacterItem) IsTemporary() bool {
	return i.Time > 0
}

// IsExpired returns true if temporary item has expired
func (i *CharacterItem) IsExpired() bool {
	if !i.IsTemporary() {
		return false
	}
	return time.Now().Unix() > int64(i.Time)
}

// GetExpirationTime returns item expiration time
func (i *CharacterItem) GetExpirationTime() *time.Time {
	if !i.IsTemporary() {
		return nil
	}
	expTime := time.Unix(int64(i.Time), 0)
	return &expTime
}

// GetPaperdollSlot returns the paperdoll slot for equipped items
func (i *CharacterItem) GetPaperdollSlot() *PaperdollSlot {
	if !i.IsEquipped() {
		return nil
	}
	slot := PaperdollSlot(i.LocData)
	return &slot
}

// GetPaperdollSlotName returns human-readable name of paperdoll slot
func (i *CharacterItem) GetPaperdollSlotName() string {
	if !i.IsEquipped() {
		return "None"
	}

	switch PaperdollSlot(i.LocData) {
	case SlotUnder:
		return "Underwear"
	case SlotHead:
		return "Helmet"
	case SlotHair:
		return "Hair"
	case SlotHair2:
		return "Hair 2"
	case SlotNeck:
		return "Necklace"
	case SlotRHand:
		return "Right Hand"
	case SlotChest:
		return "Chest"
	case SlotLHand:
		return "Left Hand"
	case SlotREar:
		return "Right Earring"
	case SlotLEar:
		return "Left Earring"
	case SlotGloves:
		return "Gloves"
	case SlotLegs:
		return "Legs"
	case SlotFeet:
		return "Feet"
	case SlotRFinger:
		return "Right Ring"
	case SlotLFinger:
		return "Left Ring"
	case SlotLBracelet:
		return "Left Bracelet"
	case SlotRBracelet:
		return "Right Bracelet"
	case SlotDeco1:
		return "Decoration 1"
	case SlotDeco2:
		return "Decoration 2"
	case SlotDeco3:
		return "Decoration 3"
	case SlotDeco4:
		return "Decoration 4"
	case SlotDeco5:
		return "Decoration 5"
	case SlotDeco6:
		return "Decoration 6"
	case SlotBack:
		return "Cloak"
	case SlotBelt:
		return "Belt"
	case SlotLRHand:
		return "Two-Handed"
	default:
		return "Unknown"
	}
}

// SetLocation moves item to specified location
func (i *CharacterItem) SetLocation(location ItemLocation, locData int) {
	i.Loc = string(location)
	i.LocData = locData
}

// Equip moves item to paperdoll slot
func (i *CharacterItem) Equip(slot PaperdollSlot) {
	i.SetLocation(LocPaperdoll, int(slot))
}

// Unequip moves item from paperdoll to inventory
func (i *CharacterItem) Unequip() {
	i.SetLocation(LocInventory, -1)
}

// SetElementalAttribute sets elemental attribute value
func (i *CharacterItem) SetElementalAttribute(attr ElementalAttribute, value int) {
	switch attr {
	case AttributeFIRE:
		i.AttributeFire = value
	case AttributeWATER:
		i.AttributeWater = value
	case AttributeWIND:
		i.AttributeWind = value
	case AttributeEARTH:
		i.AttributeEarth = value
	case AttributeHOLY:
		i.AttributeHoly = value
	case AttributeDARK:
		i.AttributeDark = value
	}
}

// Validate validates item data
func (i *CharacterItem) Validate() error {
	if i.OwnerID <= 0 {
		return ErrInvalidOwner
	}

	if i.ItemID <= 0 {
		return ErrInvalidItemID
	}

	if i.Count <= 0 {
		return ErrInvalidCount
	}

	// Validate location
	validLocs := []string{
		string(LocInventory), string(LocPaperdoll), string(LocWarehouse),
		string(LocClanWH), string(LocPet), string(LocPetEquip), string(LocFreight),
	}
	validLoc := false
	for _, loc := range validLocs {
		if i.Loc == loc {
			validLoc = true
			break
		}
	}
	if !validLoc {
		return ErrInvalidLocation
	}

	// Validate paperdoll slot
	if i.Loc == string(LocPaperdoll) {
		if i.LocData < 0 || i.LocData > 25 {
			return ErrInvalidPaperdollSlot
		}
	} else if i.LocData != -1 {
		return ErrInvalidLocationData
	}

	// Validate enchant level
	if i.EnchantLevel < 0 || i.EnchantLevel > 127 {
		return ErrInvalidEnchantLevel
	}

	return nil
}

// Item-related errors
var (
	ErrInvalidOwner         = &ItemError{"invalid owner ID"}
	ErrInvalidItemID        = &ItemError{"invalid item ID"}
	ErrInvalidCount         = &ItemError{"invalid item count"}
	ErrInvalidLocation      = &ItemError{"invalid item location"}
	ErrInvalidLocationData  = &ItemError{"invalid location data"}
	ErrInvalidPaperdollSlot = &ItemError{"invalid paperdoll slot"}
	ErrInvalidEnchantLevel  = &ItemError{"invalid enchant level"}
	ErrItemNotFound         = &ItemError{"item not found"}
	ErrInsufficientItems    = &ItemError{"insufficient items"}
	ErrSlotOccupied         = &ItemError{"equipment slot occupied"}
)

// ItemError represents item-related errors
type ItemError struct {
	msg string
}

func (e *ItemError) Error() string {
	return e.msg
}

// CharacterInventory represents a character's complete inventory
type CharacterInventory struct {
	OwnerID   int32           `json:"owner_id"`
	Items     []CharacterItem `json:"items"`
	Paperdoll []CharacterItem `json:"paperdoll"`
}

// GetInventoryItems returns items in inventory location
func (inv *CharacterInventory) GetInventoryItems() []CharacterItem {
	var items []CharacterItem
	for _, item := range inv.Items {
		if item.IsInInventory() {
			items = append(items, item)
		}
	}
	return items
}

// GetEquippedItems returns equipped items (paperdoll)
func (inv *CharacterInventory) GetEquippedItems() []CharacterItem {
	var items []CharacterItem
	for _, item := range inv.Items {
		if item.IsEquipped() {
			items = append(items, item)
		}
	}
	return items
}

// GetItemInSlot returns item equipped in specific paperdoll slot
func (inv *CharacterInventory) GetItemInSlot(slot PaperdollSlot) *CharacterItem {
	for _, item := range inv.Items {
		if item.IsEquipped() && PaperdollSlot(item.LocData) == slot {
			return &item
		}
	}
	return nil
}

// GetItemByObjectID finds item by object ID
func (inv *CharacterInventory) GetItemByObjectID(objectID int32) *CharacterItem {
	for i := range inv.Items {
		if inv.Items[i].ObjectID == objectID {
			return &inv.Items[i]
		}
	}
	return nil
}

// GetItemsByItemID returns all items with specified item template ID
func (inv *CharacterInventory) GetItemsByItemID(itemID int32) []CharacterItem {
	var items []CharacterItem
	for _, item := range inv.Items {
		if item.ItemID == itemID {
			items = append(items, item)
		}
	}
	return items
}

