package usecase

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// escapeSkillSource returns a skill carrying an "Escape" effect for the SoE id, so
// the handler recognises it as an escape scroll.
type escapeSkillSource struct{}

func (escapeSkillSource) GetSkill(id, level int) *models.Skill {
	s := &models.Skill{ID: id, Level: level}
	if id == 2013 {
		s.Effects = []models.SkillEffect{{Name: "Escape", Params: map[string]string{"escapeType": "TOWN"}}}
	}
	return s
}

func TestSkillHasEscapeEffect(t *testing.T) {
	if !skillHasEscapeEffect(&models.Skill{Effects: []models.SkillEffect{{Name: "Escape"}}}) {
		t.Error("escape skill not detected")
	}
	if skillHasEscapeEffect(&models.Skill{Effects: []models.SkillEffect{{Name: "Heal"}}}) {
		t.Error("heal skill falsely detected as escape")
	}
	if skillHasEscapeEffect(nil) {
		t.Error("nil skill should not be an escape skill")
	}
}

func TestItemSkillHandler_EscapeBlockedInCombat(t *testing.T) {
	caster := &recordingCaster{}
	itemRepo := &fakeItemRepo{}
	h := NewItemSkillHandler(escapeSkillSource{}, caster)
	repository := &fakeRepo{item: itemRepo}

	item := &models.CharacterItem{ObjectID: 700, ItemID: 736, OwnerID: 7, Count: 5}
	tmpl := &registry.ItemTemplate{
		ID: 736, Name: "Scroll of Escape", Handler: "ItemSkills",
		ItemSkills: []registry.ItemSkill{{ID: 2013, Level: 1}},
	}

	// In combat: refused — not consumed, not cast, scroll untouched.
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: item, Template: tmpl, Repo: repository, InCombat: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if consumed {
		t.Error("SoE must not be consumed while in combat")
	}
	if len(caster.calls) != 0 {
		t.Errorf("SoE must not cast while in combat, got %d casts", len(caster.calls))
	}
	if item.Count != 5 {
		t.Errorf("scroll count changed in combat: %d, want 5", item.Count)
	}

	// Out of combat: consumed + cast.
	consumed, err = h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: item, Template: tmpl, Repo: repository, InCombat: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !consumed {
		t.Error("SoE should be consumed out of combat")
	}
	if len(caster.calls) != 1 {
		t.Errorf("SoE should cast once out of combat, got %d", len(caster.calls))
	}
	if item.Count != 4 {
		t.Errorf("scroll count = %d, want 4 (one consumed)", item.Count)
	}
}

// TestItemSkillHandler_PotionUsableInCombat guards that the combat gate is escape-only:
// a healing potion (no Escape effect) still works in combat.
func TestItemSkillHandler_PotionUsableInCombat(t *testing.T) {
	h, caster, itemRepo := newItemSkillTest(map[[2]int]bool{{2037, 1}: true})
	repository := &fakeRepo{item: itemRepo}

	item := &models.CharacterItem{ObjectID: 501, ItemID: 1539, OwnerID: 7, Count: 3}
	tmpl := &registry.ItemTemplate{
		ID: 1539, Handler: "ItemSkills",
		ItemSkills: []registry.ItemSkill{{ID: 2037, Level: 1}},
	}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: item, Template: tmpl, Repo: repository, InCombat: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !consumed {
		t.Error("healing potion should be usable in combat")
	}
	if len(caster.calls) != 1 {
		t.Errorf("potion should cast in combat, got %d", len(caster.calls))
	}
}
