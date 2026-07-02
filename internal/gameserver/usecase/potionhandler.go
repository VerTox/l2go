package usecase

import (
	"context"
	"fmt"

	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// SkillEffectSource resolves an item skill (id+level) to a minimal restore effect.
// Implemented by registry.SkillEffectRegistry; abstracted here for testability.
type SkillEffectSource interface {
	Lookup(skillID, level int) (registry.SkillEffect, bool)
}

// StatRestorer applies a vital-stat restore to a live player. It is implemented on
// the game-loop side (the loop owns authoritative HP/MP/CP and broadcasts the
// resulting StatusUpdate/UserInfo), so the handler stays free of packet/world deps.
type StatRestorer interface {
	RestoreStats(charID, hp, mp, cp, skillID, skillLevel int32)
}

// PotionHandler implements ItemHandler for consumable potions whose item_skill
// points at a restore skill (handlers "ItemSkills" and "ManaPotion" in HF data).
//
// INTERIM (l2go-diu, decision b2): there is no skill engine yet, so instead of
// casting the linked skill we read its restore effect (HP/MP/CP + amount) from the
// skill data and apply it immediately, then consume one item. This will be replaced
// by a real doSimultaneousCast once the skill engine lands. Reuse timers / shared
// reuse groups (ExUseSharedGroupItem) are out of scope here — tracked by l2go-6vj.
type PotionHandler struct {
	skills  SkillEffectSource
	restore StatRestorer
}

// NewPotionHandler builds a potion handler that resolves effects via skills and
// applies restores through restore.
func NewPotionHandler(skills SkillEffectSource, restore StatRestorer) *PotionHandler {
	return &PotionHandler{skills: skills, restore: restore}
}

// UseItem resolves the item's linked restore skill(s), consumes one item and routes
// the HP/MP/CP restore to the live player. Returns consumed=false (no-op) when the
// item declares no resolvable restore effect.
func (p *PotionHandler) UseItem(ctx context.Context, use ItemUseContext) (bool, error) {
	if use.Template == nil || len(use.Template.ItemSkills) == 0 {
		return false, nil
	}

	var hp, mp, cp int32
	matched := false
	for _, sk := range use.Template.ItemSkills {
		eff, ok := p.skills.Lookup(sk.ID, sk.Level)
		if !ok {
			continue
		}
		switch eff.Kind {
		case registry.EffectHP:
			hp += int32(eff.Amount)
		case registry.EffectMP:
			mp += int32(eff.Amount)
		case registry.EffectCP:
			cp += int32(eff.Amount)
		default:
			continue
		}
		matched = true
	}
	if !matched {
		// No restore effect we understand — behave like an unhandled item (no-op),
		// so we never consume a potion without applying its effect.
		return false, nil
	}

	if err := p.consumeOne(ctx, use); err != nil {
		return false, err
	}

	// INTERIM cast visual: we don't run the real skill, but broadcasting the
	// linked skill's MagicSkillUse gives the client the cast animation and starts
	// the shortcut/reuse cooldown sweep on the item icon. Use the item's primary
	// item_skill as the cast skill.
	castID, castLvl := use.Template.ItemSkills[0].ID, use.Template.ItemSkills[0].Level
	p.restore.RestoreStats(use.CharID, hp, mp, cp, int32(castID), int32(castLvl))
	return true, nil
}

// consumeOne removes a single unit of the used item, deleting the row when the last
// unit is consumed. Mutates use.Item.Count in place so the caller can reflect the
// new count in an InventoryUpdate.
func (p *PotionHandler) consumeOne(ctx context.Context, use ItemUseContext) error {
	item := use.Item
	item.Count--
	if item.Count <= 0 {
		item.Count = 0
		if err := use.Repo.Item().Delete(ctx, item.ObjectID); err != nil {
			return fmt.Errorf("failed to delete consumed item: %w", err)
		}
		return nil
	}
	if err := use.Repo.Item().Update(ctx, item); err != nil {
		return fmt.Errorf("failed to update consumed item count: %w", err)
	}
	return nil
}
