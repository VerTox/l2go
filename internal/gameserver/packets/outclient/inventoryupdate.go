package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// Update type constants for InventoryUpdate packet
const (
	UpdateTypeAdd    int16 = 1 // Item added to inventory
	UpdateTypeModify int16 = 2 // Item modified (equip/unequip, count change)
	UpdateTypeRemove int16 = 3 // Item removed from inventory
)

// InventoryUpdate packet (opcode 0x21) - sends inventory and equipment updates
type InventoryUpdate struct {
	Items []InventoryItem `json:"items"`
}

// InventoryItem represents an item in inventory update
type InventoryItem struct {
	UpdateType   int16 `json:"update_type"` // 1=ADD, 2=MODIFY, 3=REMOVE
	ObjectID     int32 `json:"object_id"`
	ItemID       int32 `json:"item_id"`
	LocationSlot int32 `json:"location_slot"` // paperdoll index or -1
	Count        int64 `json:"count"`
	ItemType     int32 `json:"item_type"` // Type2: 0=weapon, 1=armor, 2=accessory, etc
	CustomType1  int32 `json:"custom_type1"`
	Equipped     bool  `json:"equipped"`  // Is item equipped
	BodyPart     int32 `json:"body_part"` // Equipment slot bitmask
	EnchantLevel int32 `json:"enchant_level"`
	CustomType2  int32 `json:"custom_type2"`

	// Augmentation (weapon enhancement)
	AugmentationID int32 `json:"augmentation_id"`

	// Shadow/mana
	Mana          int32 `json:"mana"`
	TimeRemaining int32 `json:"time_remaining"` // -9999 for permanent

	// Elemental attributes
	AttackElementType   int32 `json:"attack_element_type"`
	AttackElementPower  int32 `json:"attack_element_power"`
	DefenseElementFire  int32 `json:"def_element_fire"`
	DefenseElementWater int32 `json:"def_element_water"`
	DefenseElementWind  int32 `json:"def_element_wind"`
	DefenseElementEarth int32 `json:"def_element_earth"`
	DefenseElementHoly  int32 `json:"def_element_holy"`
	DefenseElementDark  int32 `json:"def_element_dark"`

	// Enchant options
	EnchantOption1 int32 `json:"enchant_option1"`
	EnchantOption2 int32 `json:"enchant_option2"`
	EnchantOption3 int32 `json:"enchant_option3"`
}

// BuildInventoryUpdate creates InventoryUpdate packet data matching L2J format
func BuildInventoryUpdate(update InventoryUpdate) []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x21) // InventoryUpdate opcode

	// Item count [H]
	b.WriteH(uint16(len(update.Items)))

	for _, item := range update.Items {
		// [H] updateType
		b.WriteH(uint16(item.UpdateType))
		// [D] objectId
		b.WriteD(int32(item.ObjectID))
		// [D] itemId
		b.WriteD(int32(item.ItemID))
		// [D] locationSlot
		b.WriteD(int32(item.LocationSlot))
		// [Q] count (8 bytes)
		b.WriteQ(int64(item.Count))
		// [H] type2
		b.WriteH(uint16(item.ItemType))
		// [H] customType1
		b.WriteH(uint16(item.CustomType1))
		// [H] isEquipped
		if item.Equipped {
			b.WriteH(0x01)
		} else {
			b.WriteH(0x00)
		}
		// [D] bodyPart
		b.WriteD(int32(item.BodyPart))
		// [H] enchantLevel
		b.WriteH(uint16(item.EnchantLevel))
		// [H] customType2
		b.WriteH(uint16(item.CustomType2))
		// [D] augmentationId
		b.WriteD(int32(item.AugmentationID))
		// [D] mana
		b.WriteD(int32(item.Mana))
		// [D] timeRemaining
		b.WriteD(int32(item.TimeRemaining))
		// [H] attackElementType
		b.WriteH(uint16(item.AttackElementType))
		// [H] attackElementPower
		b.WriteH(uint16(item.AttackElementPower))
		// [H] defFire
		b.WriteH(uint16(item.DefenseElementFire))
		// [H] defWater
		b.WriteH(uint16(item.DefenseElementWater))
		// [H] defWind
		b.WriteH(uint16(item.DefenseElementWind))
		// [H] defEarth
		b.WriteH(uint16(item.DefenseElementEarth))
		// [H] defHoly
		b.WriteH(uint16(item.DefenseElementHoly))
		// [H] defDark
		b.WriteH(uint16(item.DefenseElementDark))
		// [H] enchantOption1
		b.WriteH(uint16(item.EnchantOption1))
		// [H] enchantOption2
		b.WriteH(uint16(item.EnchantOption2))
		// [H] enchantOption3
		b.WriteH(uint16(item.EnchantOption3))
	}

	return b.Bytes()
}
