package usecase

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// ExtractableItemsHandler implements ItemHandler for extractable ("capsuled")
// items — lootboxes/capsules that yield random rewards on use.
//
// It mirrors L2J's ExtractableItems.java: the box is destroyed (1 unit), then
// every product in the item's CapsuledItems list is rolled INDEPENDENTLY. A
// product hits when rng(100000) <= product.Chance (chance stored *1000), and on
// a hit yields a random count in [Min,Max] (min==max means a fixed amount).
// Rewards are added to the character's inventory: stackable rewards merge into an
// existing stack (or create one), non-stackable rewards become separate objects.
//
// The box is always consumed even when nothing drops (L2J returns success and
// merely sends NOTHING_INSIDE_THAT), so consumed=true whenever the item was a
// valid extractable. consumed=false is returned only for items with no products.
type ExtractableItemsHandler struct {
	// rng returns a pseudo-random int in [0,n). Injected for deterministic tests;
	// defaults to math/rand.Intn (safe for concurrent use).
	rng func(n int) int
	// rate multiplies min/max reward counts (L2J RateExtractable). Default 1.0.
	rate float64
	// template resolves a reward item's static template to read its stackability.
	// Defaults to the global item-template registry; overridden in tests.
	template func(itemID int32) *registry.ItemTemplate
}

// NewExtractableItemsHandler builds the handler for the "ExtractableItems" name.
func NewExtractableItemsHandler() *ExtractableItemsHandler {
	return &ExtractableItemsHandler{
		rng:      rand.Intn,
		rate:     1.0,
		template: registry.GetItemTemplateRegistry().Get,
	}
}

// UseItem implements ItemHandler.
func (h *ExtractableItemsHandler) UseItem(ctx context.Context, use ItemUseContext) (bool, error) {
	if use.Template == nil || len(use.Template.CapsuledItems) == 0 {
		// Not a valid extractable — behave like an unhandled item (no-op), so we
		// never destroy the box without a reward table.
		return false, nil
	}

	// Destroy one box first (mirrors L2J destroyItem before rolling).
	if err := h.consumeBox(ctx, use); err != nil {
		return false, err
	}

	created := 0
	for _, p := range use.Template.CapsuledItems {
		if h.rng(100000) > p.Chance {
			continue
		}

		min := int(float64(p.Min) * h.rate)
		max := int(float64(p.Max) * h.rate)
		amount := min
		if max != min {
			amount = h.rng(max-min+1) + min
		}
		if amount <= 0 {
			continue
		}

		if err := h.giveReward(ctx, use, p.ID, amount); err != nil {
			return false, err
		}
		created++
	}

	log.Ctx(ctx).Debug().
		Int32("char_id", use.CharID).
		Int32("box_item_id", use.Template.ID).
		Int("products", len(use.Template.CapsuledItems)).
		Int("rewards", created).
		Msg("extractable item used")

	return true, nil
}

// consumeBox removes a single unit of the used box, deleting the row on the last
// unit. Mutates use.Item.Count so the dispatcher reflects the new count.
func (h *ExtractableItemsHandler) consumeBox(ctx context.Context, use ItemUseContext) error {
	item := use.Item
	item.Count--
	if item.Count <= 0 {
		item.Count = 0
		if err := use.Repo.Item().Delete(ctx, item.ObjectID); err != nil {
			return fmt.Errorf("failed to delete extracted box: %w", err)
		}
		return nil
	}
	if err := use.Repo.Item().Update(ctx, item); err != nil {
		return fmt.Errorf("failed to update extracted box count: %w", err)
	}
	return nil
}

// giveReward adds `amount` of itemID to the character's inventory and emits the
// resulting inventory change(s). Stackable rewards merge into an existing stack
// (or create a single new stack); non-stackable rewards become separate objects.
func (h *ExtractableItemsHandler) giveReward(ctx context.Context, use ItemUseContext, itemID int32, amount int) error {
	itemRepo := use.Repo.Item()

	stackable := false
	if tmpl := h.template(itemID); tmpl != nil {
		stackable = tmpl.Stackable
	}

	if stackable {
		existing, err := itemRepo.FindStackableItem(ctx, use.CharID, itemID, models.LocInventory)
		if err != nil {
			return fmt.Errorf("failed to look up stackable reward %d: %w", itemID, err)
		}
		if existing != nil {
			existing.Count += int64(amount)
			if err := itemRepo.Update(ctx, existing); err != nil {
				return fmt.Errorf("failed to update reward stack %d: %w", itemID, err)
			}
			use.emit(ChangedItem{Item: *existing, UpdateType: 2}) // MODIFY
			return nil
		}
		newItem := h.newInventoryItem(use.CharID, itemID, int64(amount))
		if err := itemRepo.Create(ctx, newItem); err != nil {
			return fmt.Errorf("failed to create reward %d: %w", itemID, err)
		}
		use.emit(ChangedItem{Item: *newItem, UpdateType: 1}) // ADD
		return nil
	}

	// Non-stackable: one object per unit.
	for i := 0; i < amount; i++ {
		newItem := h.newInventoryItem(use.CharID, itemID, 1)
		if err := itemRepo.Create(ctx, newItem); err != nil {
			return fmt.Errorf("failed to create reward object %d: %w", itemID, err)
		}
		use.emit(ChangedItem{Item: *newItem, UpdateType: 1}) // ADD
	}
	return nil
}

// newInventoryItem builds a fresh inventory item row for a reward.
func (h *ExtractableItemsHandler) newInventoryItem(charID, itemID int32, count int64) *models.CharacterItem {
	return &models.CharacterItem{
		OwnerID: charID,
		ItemID:  itemID,
		Count:   count,
		Loc:     string(models.LocInventory),
		LocData: 0,
	}
}
