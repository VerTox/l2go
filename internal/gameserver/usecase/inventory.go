package usecase

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// ChangedItem represents an item that was modified during equip/unequip operation
type ChangedItem struct {
	Item       models.CharacterItem
	UpdateType int16 // 1=add, 2=modify, 3=remove
}

// EquipResult holds the result of an equip/unequip operation
type EquipResult struct {
	ChangedItems []ChangedItem
	Success      bool
}

// InventoryUseCase handles equipment business logic
type InventoryUseCase struct {
	repo repo.DatabaseRepository
}

// NewInventoryUseCase creates a new inventory use case
func NewInventoryUseCase(repo repo.DatabaseRepository) *InventoryUseCase {
	return &InventoryUseCase{repo: repo}
}

// UseItem handles double-click on an item: equip if in inventory, unequip if equipped
func (uc *InventoryUseCase) UseItem(ctx context.Context, charID int32, objectID int32) (*EquipResult, error) {
	item, err := uc.repo.Item().GetByObjectID(ctx, objectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	if item == nil {
		return nil, fmt.Errorf("item not found: objectID=%d", objectID)
	}

	// Verify ownership
	if item.OwnerID != charID {
		return nil, fmt.Errorf("item does not belong to character")
	}

	// Get item template for body part info
	template := registry.GetItemTemplateRegistry().Get(item.ItemID)
	if template == nil {
		log.Ctx(ctx).Warn().
			Int32("item_id", item.ItemID).
			Msg("no template found for item, cannot equip")
		return &EquipResult{Success: false}, nil
	}

	// Check if item is equippable
	if template.BodyPartCode == 0 {
		log.Ctx(ctx).Debug().
			Int32("item_id", item.ItemID).
			Str("name", template.Name).
			Msg("item is not equippable (bodyPartCode=0)")
		return &EquipResult{Success: false}, nil
	}

	if item.IsEquipped() {
		// Unequip
		changed, err := uc.unequipItem(ctx, charID, item)
		if err != nil {
			return nil, fmt.Errorf("failed to unequip item: %w", err)
		}
		return &EquipResult{ChangedItems: changed, Success: true}, nil
	}

	// Equip
	changed, err := uc.equipItem(ctx, charID, item, template)
	if err != nil {
		return nil, fmt.Errorf("failed to equip item: %w", err)
	}
	return &EquipResult{ChangedItems: changed, Success: true}, nil
}

// UnequipBySlot handles dragging an item off a paperdoll slot (RequestUnEquipItem)
func (uc *InventoryUseCase) UnequipBySlot(ctx context.Context, charID int32, slotBitmask int32) (*EquipResult, error) {
	slot, ok := models.BodyPartToPaperdollSlot(slotBitmask)
	if !ok {
		log.Ctx(ctx).Warn().
			Int32("bitmask", slotBitmask).
			Msg("unknown body part bitmask for unequip")
		return &EquipResult{Success: false}, nil
	}

	item, err := uc.repo.Item().GetEquippedItem(ctx, charID, slot)
	if err != nil {
		return nil, fmt.Errorf("failed to get equipped item: %w", err)
	}
	if item == nil {
		log.Ctx(ctx).Debug().
			Int("slot", int(slot)).
			Msg("no item equipped in slot")
		return &EquipResult{Success: false}, nil
	}

	changed, err := uc.unequipItem(ctx, charID, item)
	if err != nil {
		return nil, fmt.Errorf("failed to unequip item: %w", err)
	}
	return &EquipResult{ChangedItems: changed, Success: true}, nil
}

// unequipItem moves an equipped item to inventory
func (uc *InventoryUseCase) unequipItem(ctx context.Context, charID int32, item *models.CharacterItem) ([]ChangedItem, error) {
	if !item.IsEquipped() {
		return nil, fmt.Errorf("item is not equipped")
	}

	slot := models.PaperdollSlot(item.LocData)

	log.Ctx(ctx).Info().
		Int32("char_id", charID).
		Int32("object_id", item.ObjectID).
		Int32("item_id", item.ItemID).
		Int("slot", int(slot)).
		Msg("unequipping item")

	if err := uc.repo.Item().UnequipSlot(ctx, charID, slot); err != nil {
		return nil, fmt.Errorf("failed to unequip slot %d: %w", slot, err)
	}

	// Update in-memory item state
	item.Unequip()

	return []ChangedItem{
		{Item: *item, UpdateType: 2}, // MODIFY
	}, nil
}

// equipItem equips an item, handling slot conflicts
func (uc *InventoryUseCase) equipItem(ctx context.Context, charID int32, item *models.CharacterItem, template *registry.ItemTemplate) ([]ChangedItem, error) {
	bodyPart := template.BodyPartCode
	targetSlot, ok := models.BodyPartToPaperdollSlot(bodyPart)
	if !ok {
		return nil, fmt.Errorf("cannot determine paperdoll slot for body part 0x%x", bodyPart)
	}

	var changed []ChangedItem

	// Handle slot conflicts
	switch {
	case models.IsTwoHanded(bodyPart):
		// Two-handed weapon: unequip shield (left hand) if present
		changed2, err := uc.unequipSlotIfOccupied(ctx, charID, models.SlotLHand)
		if err != nil {
			return nil, err
		}
		changed = append(changed, changed2...)
		// Also unequip existing right hand weapon
		changed2, err = uc.unequipSlotIfOccupied(ctx, charID, models.SlotRHand)
		if err != nil {
			return nil, err
		}
		changed = append(changed, changed2...)

	case models.IsFullArmor(bodyPart):
		// Full armor: unequip legs if present, equip in chest slot
		changed2, err := uc.unequipSlotIfOccupied(ctx, charID, models.SlotLegs)
		if err != nil {
			return nil, err
		}
		changed = append(changed, changed2...)
		// Also unequip existing chest armor
		changed2, err = uc.unequipSlotIfOccupied(ctx, charID, models.SlotChest)
		if err != nil {
			return nil, err
		}
		changed = append(changed, changed2...)

	case models.IsDualSlot(bodyPart):
		// Dual slot (ears, fingers, hair): try primary slot first, then secondary
		targetSlot = uc.findDualSlot(ctx, charID, bodyPart, targetSlot)
		// Unequip whatever is in the target slot
		changed2, err := uc.unequipSlotIfOccupied(ctx, charID, targetSlot)
		if err != nil {
			return nil, err
		}
		changed = append(changed, changed2...)

	default:
		// Single slot: unequip existing item if present
		// Special case: equipping one-handed weapon in right hand while two-handed is equipped
		if targetSlot == models.SlotLHand {
			// Equipping shield/left-hand item: check if right hand has two-handed weapon
			rhItem, err := uc.repo.Item().GetEquippedItem(ctx, charID, models.SlotRHand)
			if err != nil {
				return nil, err
			}
			if rhItem != nil {
				rhTemplate := registry.GetItemTemplateRegistry().Get(rhItem.ItemID)
				if rhTemplate != nil && models.IsTwoHanded(rhTemplate.BodyPartCode) {
					// Unequip two-handed weapon first
					changed2, err := uc.unequipSlotIfOccupied(ctx, charID, models.SlotRHand)
					if err != nil {
						return nil, err
					}
					changed = append(changed, changed2...)
				}
			}
		}

		changed2, err := uc.unequipSlotIfOccupied(ctx, charID, targetSlot)
		if err != nil {
			return nil, err
		}
		changed = append(changed, changed2...)
	}

	// Equip the item
	log.Ctx(ctx).Info().
		Int32("char_id", charID).
		Int32("object_id", item.ObjectID).
		Int32("item_id", item.ItemID).
		Int("target_slot", int(targetSlot)).
		Str("name", template.Name).
		Msg("equipping item")

	if err := uc.repo.Item().EquipItem(ctx, item.ObjectID, targetSlot); err != nil {
		return nil, fmt.Errorf("failed to equip item: %w", err)
	}

	// Update in-memory item state
	item.Equip(targetSlot)

	changed = append(changed, ChangedItem{
		Item:       *item,
		UpdateType: 2, // MODIFY
	})

	return changed, nil
}

// unequipSlotIfOccupied unequips the item in the given slot if one exists
func (uc *InventoryUseCase) unequipSlotIfOccupied(ctx context.Context, charID int32, slot models.PaperdollSlot) ([]ChangedItem, error) {
	existing, err := uc.repo.Item().GetEquippedItem(ctx, charID, slot)
	if err != nil {
		return nil, fmt.Errorf("failed to check slot %d: %w", slot, err)
	}
	if existing == nil {
		return nil, nil
	}

	return uc.unequipItem(ctx, charID, existing)
}

// findDualSlot determines which slot to use for a dual-slot item (ears, fingers)
func (uc *InventoryUseCase) findDualSlot(ctx context.Context, charID int32, bodyPart int32, primarySlot models.PaperdollSlot) models.PaperdollSlot {
	var secondarySlot models.PaperdollSlot

	switch bodyPart {
	case models.BodyPartLREar:
		secondarySlot = models.SlotLEar
	case models.BodyPartLRFinger:
		secondarySlot = models.SlotLFinger
	case models.BodyPartHairAll:
		secondarySlot = models.SlotHair2
	default:
		return primarySlot
	}

	// Check if primary slot is free
	primary, err := uc.repo.Item().GetEquippedItem(ctx, charID, primarySlot)
	if err != nil || primary == nil {
		return primarySlot // Primary slot free or error — use primary
	}

	// Primary occupied, check secondary
	secondary, err := uc.repo.Item().GetEquippedItem(ctx, charID, secondarySlot)
	if err != nil || secondary == nil {
		return secondarySlot // Secondary slot free — use it
	}

	// Both occupied — replace primary
	return primarySlot
}
