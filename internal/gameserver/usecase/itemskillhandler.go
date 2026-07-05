package usecase

import (
	"context"
	"fmt"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// ItemSkillCaster triggers a real cast of an item's linked skill (id+level) through
// the skill engine. Implemented on the game-loop side (it owns casting/vitals state
// and broadcasts the animation), so the handler stays free of packet/world deps.
type ItemSkillCaster interface {
	CastItemSkill(charID, skillID, level int32)
}

// SkillTemplateSource resolves a skill template by (id, level). Implemented by
// registry.SkillData; used here only to validate that the potion's linked skill
// exists before consuming the item.
type SkillTemplateSource interface {
	GetSkill(skillID, level int) *models.Skill
}

// ItemSkillHandler implements ItemHandler for any consumable whose use casts a
// linked item_skill — the HF datapack handlers "ItemSkills" and "ManaPotion" (not
// just potions: healing/mana potions, Scroll of Escape, and other skill-consumables
// all route here). It casts that skill through the real skill engine (restore
// effects, buffs/HoT via the continuous-effect engine, Escape → teleport), then
// consumes one item. (l2go-849, replaces the interim direct-restore path l2go-diu;
// renamed from PotionHandler as its scope was never potion-only.)
//
// Item reuse timers / shared reuse groups (ExUseSharedGroupItem) are handled by the
// inventory use case off item consumption, not here.
//
// The escape-in-combat gate here is a stop-gap: the general model is a skill <cond>
// system (L2J canEscape/insideZone), which the skill engine doesn't parse yet — so
// this one item-category rule is hardcoded until conditions land (l2go-z36.1).
type ItemSkillHandler struct {
	skills SkillTemplateSource
	caster ItemSkillCaster
}

// NewItemSkillHandler builds a potion handler that validates skills via skills and
// casts them through caster.
func NewItemSkillHandler(skills SkillTemplateSource, caster ItemSkillCaster) *ItemSkillHandler {
	return &ItemSkillHandler{skills: skills, caster: caster}
}

// skillHasEscapeEffect reports whether the skill teleports the user (Scroll of
// Escape and kin), so it can be blocked in combat. (l2go-kg9)
func skillHasEscapeEffect(skill *models.Skill) bool {
	if skill == nil {
		return false
	}
	for _, eff := range skill.Effects {
		if eff.Name == "Escape" {
			return true
		}
	}
	return false
}

// UseItem casts the item's linked skill(s) through the skill engine and consumes one
// item. Returns consumed=false (no-op) when the item declares no resolvable skill,
// so a potion is never consumed without an effect.
func (p *ItemSkillHandler) UseItem(ctx context.Context, use ItemUseContext) (bool, error) {
	if use.Template == nil || len(use.Template.ItemSkills) == 0 {
		return false, nil
	}

	// Collect the skills that actually resolve in the datapack; a potion pointing at
	// only unknown skills behaves like an unhandled item (no-op, not consumed).
	type cast struct{ id, level int32 }
	var casts []cast
	for _, sk := range use.Template.ItemSkills {
		skill := p.skills.GetSkill(sk.ID, sk.Level)
		if p.skills != nil && skill == nil {
			continue
		}
		// Escape scrolls (effect "Escape") can't be used in combat (L2J canEscape).
		// Refuse before consuming so a blocked attempt doesn't waste the scroll. Other
		// consumables (potions) are fine in combat. (l2go-kg9)
		if use.InCombat && skillHasEscapeEffect(skill) {
			return false, nil
		}
		casts = append(casts, cast{id: int32(sk.ID), level: int32(sk.Level)})
	}
	if len(casts) == 0 {
		return false, nil
	}

	if err := p.consumeOne(ctx, use); err != nil {
		return false, err
	}

	for _, c := range casts {
		p.caster.CastItemSkill(use.CharID, c.id, c.level)
	}
	return true, nil
}

// consumeOne removes a single unit of the used item, deleting the row when the last
// unit is consumed. Mutates use.Item.Count in place so the caller can reflect the
// new count in an InventoryUpdate.
func (p *ItemSkillHandler) consumeOne(ctx context.Context, use ItemUseContext) error {
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
