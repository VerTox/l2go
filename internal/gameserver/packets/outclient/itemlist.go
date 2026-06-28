package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// ItemList packet (opcode 0x11) - sends complete inventory at world entry
type ItemList struct {
	ShowWindow bool        `json:"show_window"`
	Items      []ItemEntry `json:"items"`
}

// ItemEntry represents an item in ItemList packet
type ItemEntry struct {
	ObjectID     int32 `json:"object_id"`
	ItemID       int32 `json:"item_id"`
	LocationSlot int32 `json:"location_slot"` // PAPERDOLL slot for equipped, -1 for inventory
	Count        int64 `json:"count"`
	ItemType     int32 `json:"item_type"`     // Type2: 0=weapon, 1=armor, 2=accessory, 3=quest, 4=money, 5=other
	Equipped     bool  `json:"equipped"`
	BodyPart     int32 `json:"body_part"`     // Body part bitmask from item template
	EnchantLevel int32 `json:"enchant_level"`
	CustomType1  int32 `json:"custom_type1"`
	CustomType2  int32 `json:"custom_type2"`

	// Augmentation
	AugmentationID int32 `json:"augmentation_id"`
	Mana           int32 `json:"mana"`

	// Remaining time for temporary items
	RemainingTime int32 `json:"remaining_time"`

	// Elemental attributes
	AttackElementType   int32 `json:"attack_element_type"`
	AttackElementPower  int32 `json:"attack_element_power"`
	DefenseElementFire  int32 `json:"def_element_fire"`
	DefenseElementWater int32 `json:"def_element_water"`
	DefenseElementWind  int32 `json:"def_element_wind"`
	DefenseElementEarth int32 `json:"def_element_earth"`
	DefenseElementHoly  int32 `json:"def_element_holy"`
	DefenseElementDark  int32 `json:"def_element_dark"`

	// Enchant options (3 slots)
	EnchantOptions [3]int32 `json:"enchant_options"`
}

func (p ItemList) Write(w *l2pkt.Writer) {
	w.WriteC(0x11)
	w.WriteH(boolToUInt16(p.ShowWindow))
	w.WriteH(uint16(len(p.Items)))

	// Write each item
	for _, item := range p.Items {
		// Item identification
		w.WriteD(item.ObjectID)
		w.WriteD(item.ItemID)
		w.WriteD(item.LocationSlot) // Location slot: PAPERDOLL slot for equipped, -1 for inventory
		w.WriteQ(item.Count)

		// Item type and custom fields
		w.WriteH(uint16(item.ItemType))
		w.WriteH(uint16(item.CustomType1))
		w.WriteH(boolToUInt16(item.Equipped))
		w.WriteD(item.BodyPart)

		// Enchantment
		w.WriteH(uint16(item.EnchantLevel))
		w.WriteH(uint16(item.CustomType2))
		w.WriteD(item.AugmentationID)
		w.WriteD(item.Mana)

		// Remaining time for temporary items
		w.WriteD(item.RemainingTime)

		// Elemental attributes
		w.WriteH(uint16(item.AttackElementType))
		w.WriteH(uint16(item.AttackElementPower))
		w.WriteH(uint16(item.DefenseElementFire))
		w.WriteH(uint16(item.DefenseElementWater))
		w.WriteH(uint16(item.DefenseElementWind))
		w.WriteH(uint16(item.DefenseElementEarth))
		w.WriteH(uint16(item.DefenseElementHoly))
		w.WriteH(uint16(item.DefenseElementDark))

		// Enchant options (3 slots)
		for _, option := range item.EnchantOptions {
			w.WriteH(uint16(option))
		}
	}

	// Inventory block (simplified for now)
	w.WriteH(0x00) // No blocked items
}

// boolToUInt16 converts boolean to uint16 (0 or 1)
func boolToUInt16(b bool) uint16 {
	if b {
		return 1
	}
	return 0
}
