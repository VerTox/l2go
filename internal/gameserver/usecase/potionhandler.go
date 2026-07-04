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

// PotionHandler implements ItemHandler for consumable potions whose item_skill
// points at a skill (handlers "ItemSkills" and "ManaPotion" in HF data). It casts
// that skill through the real skill engine (restore effects, and buffs/HoT via the
// continuous-effect engine), then consumes one item. (l2go-849, replaces the interim
// direct-restore path l2go-diu.)
//
// Item reuse timers / shared reuse groups (ExUseSharedGroupItem) are handled by the
// inventory use case off item consumption, not here.
type PotionHandler struct {
	skills SkillTemplateSource
	caster ItemSkillCaster
}

// NewPotionHandler builds a potion handler that validates skills via skills and
// casts them through caster.
func NewPotionHandler(skills SkillTemplateSource, caster ItemSkillCaster) *PotionHandler {
	return &PotionHandler{skills: skills, caster: caster}
}

// UseItem casts the item's linked skill(s) through the skill engine and consumes one
// item. Returns consumed=false (no-op) when the item declares no resolvable skill,
// so a potion is never consumed without an effect.
func (p *PotionHandler) UseItem(ctx context.Context, use ItemUseContext) (bool, error) {
	if use.Template == nil || len(use.Template.ItemSkills) == 0 {
		return false, nil
	}

	// Collect the skills that actually resolve in the datapack; a potion pointing at
	// only unknown skills behaves like an unhandled item (no-op, not consumed).
	type cast struct{ id, level int32 }
	var casts []cast
	for _, sk := range use.Template.ItemSkills {
		if p.skills != nil && p.skills.GetSkill(sk.ID, sk.Level) == nil {
			continue
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
