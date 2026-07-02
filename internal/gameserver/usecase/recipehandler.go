package usecase

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// Recipe registration constants, mirroring L2J.
const (
	// Create Item skill ids gating recipe registration (CommonSkill.CREATE_DWARVEN /
	// CREATE_COMMON). A character must know the matching skill to register a recipe.
	createDwarvenSkillID int32 = 172
	createCommonSkillID  int32 = 1320

	// defaultRecipeLimit is L2J's DwarfRecipeLimit / CommonRecipeLimit config default
	// (character.properties = 50/50). We have no stat engine to add REC_D_LIM/REC_C_LIM
	// bonuses yet, so both books use this flat cap. (decision: config-default limit)
	defaultRecipeLimit = 50

	// SystemMessage ids used by the recipe handler.
	sysMsgCreateLvlTooLow         int32 = 404  // CREATE_LVL_TOO_LOW_TO_REGISTER
	sysMsgRecipeAlreadyRegistered int32 = 840  // RECIPE_ALREADY_REGISTERED
	sysMsgS1Added                 int32 = 851  // S1_ADDED ("$s1 has been added.")
	sysMsgUpToS1Recipes           int32 = 894  // UP_TO_S1_RECIPES_CAN_REGISTER
	sysMsgCantRegisterNoAbility   int32 = 1061 // CANT_REGISTER_NO_ABILITY_TO_CRAFT
)

// RecipeSource resolves a recipe scroll's item id to the recipe it registers.
// Implemented by registry.RecipeRegistry; abstracted here for testability.
type RecipeSource interface {
	GetByItemID(itemID int32) (*registry.Recipe, bool)
}

// RecipeNotifier delivers the client-visible feedback of recipe registration
// (system messages to the owner). It is decoupled from the domain handler so the
// registration logic stays testable without a network/world dependency. A nil
// notifier is valid — the handler then registers silently.
type RecipeNotifier interface {
	// SystemMessage sends a parameterless system message (e.g. RECIPE_ALREADY_REGISTERED).
	SystemMessage(charID int32, msgID int32)
	// SystemMessageWithInt sends a system message with a single int parameter
	// (e.g. UP_TO_S1_RECIPES_CAN_REGISTER carrying the recipe limit).
	SystemMessageWithInt(charID int32, msgID int32, value int32)
	// ItemSystemMessage sends a system message carrying an item-name parameter
	// (e.g. S1_ADDED "$s1 has been added.").
	ItemSystemMessage(charID int32, msgID int32, itemID int32)
}

// RecipeHandler implements ItemHandler for recipe scrolls (item handler "Recipes").
// It mirrors L2J's handlers.itemhandlers.Recipes: resolve the recipe by the scroll's
// item id, reject if already registered / craft ability missing / craft level too low
// / recipe book full, otherwise register the recipe (dwarven vs common book) and
// consume one scroll.
//
// Faithful to L2J, registration requires the matching Create Item skill (Create
// Dwarven Item = 172 for dwarven recipes, Create Common Item = 1320 for common):
// a character without that skill gets CANT_REGISTER_NO_ABILITY_TO_CRAFT. Unlike
// L2J, no RecipeBookItemList packet is pushed on success — the client refreshes the
// book itself on open; L2J's Recipes handler likewise only sends the S1_ADDED
// SystemMessage. The consumed scroll is reflected via the caller's InventoryUpdate.
type RecipeHandler struct {
	recipes  RecipeSource
	notifier RecipeNotifier // may be nil
	// per-book registration caps (default L2J config value).
	dwarvenLimit int
	commonLimit  int
}

// NewRecipeHandler builds the ItemHandler for the "Recipes" item handler name.
func NewRecipeHandler(recipes RecipeSource, notifier RecipeNotifier) *RecipeHandler {
	return &RecipeHandler{
		recipes:      recipes,
		notifier:     notifier,
		dwarvenLimit: defaultRecipeLimit,
		commonLimit:  defaultRecipeLimit,
	}
}

// UseItem implements ItemHandler.
func (h *RecipeHandler) UseItem(ctx context.Context, use ItemUseContext) (bool, error) {
	// 1. Resolve the recipe by the scroll's item id (L2J getRecipeByItemId).
	rp, ok := h.recipes.GetByItemID(use.Item.ItemID)
	if !ok {
		// Not a known recipe scroll (or recipes.xml missing) — no-op, like an
		// unhandled item. Never consume something we do not understand.
		return false, nil
	}

	// 2. Already registered? Refuse without consuming.
	has, err := use.Repo.Recipe().HasRecipe(ctx, use.CharID, rp.ID)
	if err != nil {
		return false, fmt.Errorf("failed to check recipe registration: %w", err)
	}
	if has {
		h.notify(func(n RecipeNotifier) { n.SystemMessage(use.CharID, sysMsgRecipeAlreadyRegistered) })
		return false, nil
	}

	// 3. Craft-ability + craft-level gating (per book).
	craftSkillID := createCommonSkillID
	limit := h.commonLimit
	if rp.IsDwarven {
		craftSkillID = createDwarvenSkillID
		limit = h.dwarvenLimit
	}

	craftLevel, err := use.Repo.Skill().GetSkillLevel(ctx, use.CharID, craftSkillID)
	if err != nil {
		return false, fmt.Errorf("failed to read craft skill level: %w", err)
	}
	if craftLevel < 1 {
		h.notify(func(n RecipeNotifier) { n.SystemMessage(use.CharID, sysMsgCantRegisterNoAbility) })
		return false, nil
	}
	if rp.CraftLevel > craftLevel {
		h.notify(func(n RecipeNotifier) { n.SystemMessage(use.CharID, sysMsgCreateLvlTooLow) })
		return false, nil
	}

	// 4. Per-book recipe limit.
	count, err := use.Repo.Recipe().CountByType(ctx, use.CharID, rp.IsDwarven)
	if err != nil {
		return false, fmt.Errorf("failed to count registered recipes: %w", err)
	}
	if count >= limit {
		h.notify(func(n RecipeNotifier) { n.SystemMessageWithInt(use.CharID, sysMsgUpToS1Recipes, int32(limit)) })
		return false, nil
	}

	// 5. Register the recipe.
	if err := use.Repo.Recipe().AddRecipe(ctx, use.CharID, rp.ID, rp.IsDwarven); err != nil {
		return false, fmt.Errorf("failed to register recipe: %w", err)
	}

	// 6. Consume one scroll.
	if err := h.consumeOne(ctx, use); err != nil {
		return false, err
	}

	// 7. Confirmation message (L2J S1_ADDED with the scroll's item name).
	h.notify(func(n RecipeNotifier) { n.ItemSystemMessage(use.CharID, sysMsgS1Added, use.Item.ItemID) })

	log.Ctx(ctx).Debug().
		Int32("char_id", use.CharID).
		Int32("recipe_id", rp.ID).
		Int32("scroll_item_id", use.Item.ItemID).
		Bool("dwarven", rp.IsDwarven).
		Msg("recipe registered")

	return true, nil
}

// consumeOne removes a single scroll, deleting the row when the last one is used.
// Mutates use.Item.Count in place so the caller reflects the new count/removal in
// the InventoryUpdate.
func (h *RecipeHandler) consumeOne(ctx context.Context, use ItemUseContext) error {
	item := use.Item
	item.Count--
	if item.Count <= 0 {
		item.Count = 0
		if err := use.Repo.Item().Delete(ctx, item.ObjectID); err != nil {
			return fmt.Errorf("failed to delete consumed recipe scroll: %w", err)
		}
		return nil
	}
	if err := use.Repo.Item().Update(ctx, item); err != nil {
		return fmt.Errorf("failed to update recipe scroll count: %w", err)
	}
	return nil
}

func (h *RecipeHandler) notify(fn func(RecipeNotifier)) {
	if h.notifier != nil {
		fn(h.notifier)
	}
}
