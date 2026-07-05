package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// SystemMessage ids used by the UseItem pre-fork guards (mirror L2J
// SystemMessageId.java, values verified against the HF reference).
const (
	sysMsgS1CannotBeUsed      = 113  // S1_CANNOT_BE_USED
	sysMsgCannotUseQuestItems = 148  // CANNOT_USE_QUEST_ITEMS
	sysMsgSecReuseS1          = 2303 // S2_SECONDS_REMAINING_FOR_REUSE_S1
	sysMsgMinSecReuseS1       = 2304 // S2_MINUTES_S3_SECONDS_REMAINING_FOR_REUSE_S1
	sysMsgHourMinSecReuseS1   = 2305 // S2_HOURS_S3_MINUTES_S4_SECONDS_REMAINING_FOR_REUSE_S1
)

// ChangedItem represents an item that was modified during equip/unequip operation
type ChangedItem struct {
	Item       models.CharacterItem
	UpdateType int16 // 1=add, 2=modify, 3=remove
}

// SysMsgSpec is a system-message directive the transport layer must deliver to
// the acting player. Keeping it a plain data struct lets UseItem stay free of
// packet/transport dependencies while still driving the client feedback that L2J
// sends inline (quest-item refusal, dead refusal, reuse-remaining).
type SysMsgSpec struct {
	ID       int32   // SystemMessageId
	ItemName int32   // when >0, appended first as an item-name parameter (S1)
	Ints     []int32 // appended after ItemName, in order
}

// ReuseSyncSpec drives an ExUseSharedGroupItem packet: it syncs a shared-reuse
// group cooldown to the client's item icon. Emitted both when a use is refused
// during cooldown and when a successful use arms one. Only produced for items
// that declare a shared reuse group (GroupID>0), matching L2J sendSharedGroupUpdate.
type ReuseSyncSpec struct {
	ItemID    int32
	GroupID   int32
	Remaining time.Duration
	Total     time.Duration
}

// PlayerCondition carries the volatile actor state UseItem must consult before
// acting on an item, mirroring the L2J UseItem.runImpl guards. Only IsDead is
// currently modelled; stun/sleep/afraid/alikeDead states do not yet exist in our
// actor model (parked — see l2go-5i0).
type PlayerCondition struct {
	IsDead   bool
	InCombat bool // blocks escape-scroll use (l2go-kg9); potions etc. are unaffected
}

// EquipResult holds the result of an equip/unequip/use operation.
type EquipResult struct {
	ChangedItems []ChangedItem
	Success      bool

	// Messages are system messages the caller must deliver to the acting player,
	// regardless of Success (quest-item / dead / reuse-remaining refusals produce
	// them with Success=false; a successful use produces none).
	Messages []SysMsgSpec

	// ReuseSync, when non-nil, is an ExUseSharedGroupItem the caller must send to
	// sync the shared-group cooldown icon (on refusal during cooldown, and on a
	// successful arm of a grouped consumable).
	ReuseSync *ReuseSyncSpec
}

// InventoryUseCase handles equipment business logic
type InventoryUseCase struct {
	repo         repo.DatabaseRepository
	itemHandlers *ItemHandlerRegistry

	// reuse holds per-character item reuse cooldowns (l2go-6vj).
	reuse *registry.ItemReuseRegistry
	// now is the injectable clock used for cooldown math (defaults to time.Now).
	now func() time.Time
	// templateOf resolves an item's static template (defaults to the global item
	// template registry; overridden in tests).
	templateOf func(itemID int32) *registry.ItemTemplate
}

// NewInventoryUseCase creates a new inventory use case
func NewInventoryUseCase(repo repo.DatabaseRepository) *InventoryUseCase {
	return &InventoryUseCase{
		repo:         repo,
		itemHandlers: NewItemHandlerRegistry(),
		reuse:        registry.GetItemReuseRegistry(),
		now:          time.Now,
		templateOf:   registry.GetItemTemplateRegistry().Get,
	}
}

// ItemHandlers exposes the item handler registry so downstream wiring can
// register concrete handlers (soulshots, potions, enchant scrolls, ...).
func (uc *InventoryUseCase) ItemHandlers() *ItemHandlerRegistry {
	return uc.itemHandlers
}

// UseItem handles double-click on an item: equip if in inventory, unequip if
// equipped, or dispatch a non-equipment item to its handler. Before the
// equip-vs-etc fork it applies the shared L2J UseItem.runImpl guards
// (quest-item, dead, reuse cooldown); cond supplies the caller-owned actor state.
func (uc *InventoryUseCase) UseItem(ctx context.Context, charID int32, objectID int32, cond PlayerCondition) (*EquipResult, error) {
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
	template := uc.templateOf(item.ItemID)
	if template == nil {
		log.Ctx(ctx).Warn().
			Int32("item_id", item.ItemID).
			Msg("no template found for item, cannot equip")
		return &EquipResult{Success: false}, nil
	}

	// --- Shared pre-fork checks (mirror L2J UseItem.runImpl, applied before the
	// equip-vs-etc fork so they cover both equipment and consumables) ---

	// Quest items can never be "used" (L2J: type2 == QUEST -> CANNOT_USE_QUEST_ITEMS).
	if template.Type2 == registry.ItemType2Quest {
		return &EquipResult{
			Success:  false,
			Messages: []SysMsgSpec{{ID: sysMsgCannotUseQuestItems}},
		}, nil
	}

	// Dead actor cannot use items (L2J: isDead -> S1_CANNOT_BE_USED). Stun/sleep/
	// afraid/alikeDead are not modelled yet (parked).
	if cond.IsDead {
		return &EquipResult{
			Success:  false,
			Messages: []SysMsgSpec{{ID: sysMsgS1CannotBeUsed, ItemName: item.ItemID}},
		}, nil
	}

	// Reuse cooldown: if the item (or its shared reuse group) is still on cooldown,
	// refuse the use and tell the client the remaining time (L2J: getItemRemaining
	// ReuseTime / getReuseDelayOnGroup -> reuseData + sendSharedGroupUpdate).
	if reuse := time.Duration(template.ReuseDelay) * time.Millisecond; reuse > 0 {
		now := uc.now()
		if rem := uc.reuse.RemainingByItem(charID, objectID, now); rem > 0 {
			return uc.reuseDenied(item.ItemID, template.SharedReuseGroup, rem, reuse), nil
		}
		if rem := uc.reuse.RemainingByGroup(charID, template.SharedReuseGroup, now); rem > 0 {
			return uc.reuseDenied(item.ItemID, template.SharedReuseGroup, rem, reuse), nil
		}
	}

	// Non-equipment item: dispatch to a registered item handler by name.
	// Mirrors L2J's ItemHandler lookup on L2EtcItem.getHandlerName().
	if template.BodyPartCode == 0 {
		return uc.useNonEquipItem(ctx, charID, item, template, cond.InCombat)
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

// useNonEquipItem dispatches the use of a non-equipment item to a registered
// ItemHandler keyed by template.Handler. If no handler is registered (or the
// item declares no handler), this is a no-op and NOT an error — exactly like
// L2J, where a missing handler simply means the item does nothing on use.
func (uc *InventoryUseCase) useNonEquipItem(ctx context.Context, charID int32, item *models.CharacterItem, template *registry.ItemTemplate, inCombat bool) (*EquipResult, error) {
	handler, ok := uc.itemHandlers.Get(template.Handler)
	if !ok {
		log.Ctx(ctx).Debug().
			Int32("item_id", item.ItemID).
			Str("name", template.Name).
			Str("handler", template.Handler).
			Msg("no item handler registered, ignoring use (no-op)")
		return &EquipResult{Success: false}, nil
	}

	// Collect inventory changes the handler produces for items other than the
	// used one (e.g. extractable rewards), so they ride along the same
	// InventoryUpdate as the used item's own change.
	var extraChanges []ChangedItem
	consumed, err := handler.UseItem(ctx, ItemUseContext{
		CharID:   charID,
		Item:     item,
		Template: template,
		Repo:     uc.repo,
		InCombat: inCombat,
		Emit:     func(ci ChangedItem) { extraChanges = append(extraChanges, ci) },
	})
	if err != nil {
		return nil, fmt.Errorf("item handler %q failed: %w", template.Handler, err)
	}

	log.Ctx(ctx).Debug().
		Int32("item_id", item.ItemID).
		Str("handler", template.Handler).
		Bool("consumed", consumed).
		Msg("item handler invoked")

	if !consumed {
		return &EquipResult{Success: false}, nil
	}

	// Reflect the post-use item state to the client. Handlers that consume stock
	// (potions, soulshots) mutate item.Count in place; a fully-consumed stack
	// (Count<=0) is reported as a removal, otherwise as a count modification.
	updateType := int16(2) // MODIFY
	if item.Count <= 0 {
		updateType = 3 // REMOVE
	}
	changed := make([]ChangedItem, 0, 1+len(extraChanges))
	changed = append(changed, ChangedItem{Item: *item, UpdateType: updateType})
	changed = append(changed, extraChanges...)

	result := &EquipResult{
		Success:      true,
		ChangedItems: changed,
	}

	// Arm the per-character reuse cooldown (l2go-6vj) now that the item was
	// actually used, mirroring L2J UseItem: addTimeStampItem(item, reuseDelay) +
	// sendSharedGroupUpdate on success. Keyed by item objectID, group-aware.
	if reuse := time.Duration(template.ReuseDelay) * time.Millisecond; reuse > 0 {
		uc.reuse.Add(charID, item.ObjectID, template.SharedReuseGroup, reuse, uc.now())
		if template.SharedReuseGroup > 0 {
			result.ReuseSync = &ReuseSyncSpec{
				ItemID:    item.ItemID,
				GroupID:   int32(template.SharedReuseGroup),
				Remaining: reuse,
				Total:     reuse,
			}
		}
	}

	return result, nil
}

// reuseDenied builds the refusal result for a use blocked by an active reuse
// cooldown: a reuse-remaining system message plus, for grouped items, an
// ExUseSharedGroupItem to keep the client cooldown icon in sync.
func (uc *InventoryUseCase) reuseDenied(itemID int32, group int, remaining, total time.Duration) *EquipResult {
	res := &EquipResult{
		Success:  false,
		Messages: []SysMsgSpec{reuseSysMsg(itemID, remaining)},
	}
	if group > 0 {
		res.ReuseSync = &ReuseSyncSpec{
			ItemID:    itemID,
			GroupID:   int32(group),
			Remaining: remaining,
			Total:     total,
		}
	}
	return res
}

// reuseSysMsg selects the correct "remaining for reuse" system message and packs
// its hours/minutes/seconds parameters, mirroring L2J UseItem.reuseData.
func reuseSysMsg(itemID int32, remaining time.Duration) SysMsgSpec {
	ms := remaining.Milliseconds()
	hours := int32(ms / 3600000)
	minutes := int32((ms % 3600000) / 60000)
	seconds := int32((ms / 1000) % 60)
	switch {
	case hours > 0:
		return SysMsgSpec{ID: sysMsgHourMinSecReuseS1, ItemName: itemID, Ints: []int32{hours, minutes, seconds}}
	case minutes > 0:
		return SysMsgSpec{ID: sysMsgMinSecReuseS1, ItemName: itemID, Ints: []int32{minutes, seconds}}
	default:
		return SysMsgSpec{ID: sysMsgSecReuseS1, ItemName: itemID, Ints: []int32{seconds}}
	}
}

// autoSoulShotHandler is the template.Handler name of the physical soulshot item
// handler — the only shot type an auto-recharge arms on a physical swing
// (spiritshots are for magic/skills, mirroring L2J rechargeShots(physical=true)).
const autoSoulShotHandler = "SoulShots"

// RechargeAutoShots charges the equipped weapon from the character's active
// auto-shots (physical soulshots only). It mirrors L2J rechargeShots(physical):
// resolve each active shot in inventory, run its handler in Auto mode (consume +
// charge + visual, no chat spam). Returns the shot stacks it actually consumed (so
// the caller can push an InventoryUpdate to refresh the count) and the item ids it
// auto-disabled because the player ran out (so the caller can echo
// ExAutoSoulShot(off)). Runs off the game loop, never on the tick. (l2go-btb)
func (uc *InventoryUseCase) RechargeAutoShots(ctx context.Context, charID int32) (consumed []ChangedItem, disabled []int32, err error) {
	active := registry.GetAutoShotRegistry().List(charID)
	if len(active) == 0 {
		return nil, nil, nil
	}

	items, err := uc.repo.Item().GetByCharacter(ctx, charID)
	if err != nil {
		return nil, nil, err
	}

	for _, itemID := range active {
		tmpl := registry.GetItemTemplateRegistry().Get(itemID)
		if tmpl == nil || tmpl.Handler != autoSoulShotHandler {
			continue // not a physical soulshot — skip on a physical swing
		}

		stack := findItemStack(items, itemID)
		if stack == nil {
			// Out of this shot entirely: drop the auto-shot (L2J rechargeShots does
			// the same via removeAutoSoulShot on a missing item).
			registry.GetAutoShotRegistry().Disable(charID, itemID)
			disabled = append(disabled, itemID)
			continue
		}

		handler, ok := uc.ItemHandlers().Get(tmpl.Handler)
		if !ok {
			continue
		}
		did, uerr := handler.UseItem(ctx, ItemUseContext{
			CharID: charID, Item: stack, Template: tmpl, Repo: uc.repo, Auto: true,
		})
		if uerr != nil {
			log.Ctx(ctx).Error().Err(uerr).
				Int32("char_id", charID).Int32("item_id", itemID).
				Msg("auto-shot recharge failed")
			continue
		}
		if did {
			// The handler mutates stack.Count in place (0 when the last shot emptied
			// and deleted the stack). Report MODIFY, or REMOVE when the stack is gone.
			updateType := int16(2) // MODIFY
			if stack.Count <= 0 {
				updateType = 3 // REMOVE
			}
			consumed = append(consumed, ChangedItem{Item: *stack, UpdateType: updateType})
		}
	}
	return consumed, disabled, nil
}

// findItemStack returns the first owned stack of itemID with a positive count.
func findItemStack(items []models.CharacterItem, itemID int32) *models.CharacterItem {
	for i := range items {
		if items[i].ItemID == itemID && items[i].Count > 0 {
			return &items[i]
		}
	}
	return nil
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

	// A soulshot/spiritshot charge belongs to the equipped weapon instance and
	// must not survive an unequip (L2J clears it on weapon change). Only weapons
	// (right hand, two-handed included) ever hold charges; clearing any other
	// object id is a harmless no-op. (l2go-77a)
	if slot == models.SlotRHand {
		registry.GetChargedShotRegistry().Clear(item.ObjectID)
	}

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
