package usecase

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// ShotEffectNotifier delivers the client-visible effects of (de)charging a shot:
// system messages to the owner and the MagicSkillUse activation animation to the
// owner and nearby players. It is intentionally decoupled from the domain
// handler so the shot logic stays testable without a network/world dependency.
//
// A nil notifier is valid — the handler then performs the domain state changes
// (consume + charge) silently, which is what the unit tests rely on.
type ShotEffectNotifier interface {
	// ItemSystemMessage sends a system message carrying an item-name parameter
	// (e.g. USE_S1_ "You are using $s1.").
	ItemSystemMessage(charID int32, msgID int32, itemID int32)
	// SystemMessage sends a parameterless system message (e.g. ENABLED_SOULSHOT,
	// or a failure message such as SOULSHOTS_GRADE_MISMATCH).
	SystemMessage(charID int32, msgID int32)
	// BroadcastShotVisual broadcasts the shot activation animation (MagicSkillUse)
	// to the owner and nearby players.
	BroadcastShotVisual(charID int32, skillID int32, skillLevel int32)
}

// shotMessages holds the shot-type-specific SystemMessage ids so a single
// handler implementation serves both soulshots and spiritshots.
type shotMessages struct {
	cannotUse     int32
	gradeMismatch int32
	notEnough     int32
	enabled       int32
	useItem       int32
}

// shotHandler implements ItemHandler for soulshots and spiritshots. It mirrors
// L2J's SoulShots.java / SpiritShot.java: validate the active weapon, match the
// grade, refuse to double-charge, consume the shots and charge the weapon
// instance, then broadcast the activation visual.
type shotHandler struct {
	shot     registry.ShotType
	charged  *registry.ChargedShotRegistry
	notifier ShotEffectNotifier // may be nil
	msgs     shotMessages
	// weaponTemplate resolves a weapon item's static template. Defaults to the
	// global item-template registry; overridden in tests.
	weaponTemplate func(itemID int32) *registry.ItemTemplate
}

// NewSoulShotHandler builds the ItemHandler for the "SoulShots" item handler name.
func NewSoulShotHandler(charged *registry.ChargedShotRegistry, notifier ShotEffectNotifier) ItemHandler {
	return &shotHandler{
		shot:     registry.ShotSoulshot,
		charged:  charged,
		notifier: notifier,
		msgs: shotMessages{
			cannotUse:     339, // CANNOT_USE_SOULSHOTS
			gradeMismatch: 337, // SOULSHOTS_GRADE_MISMATCH
			notEnough:     338, // NOT_ENOUGH_SOULSHOTS
			enabled:       342, // ENABLED_SOULSHOT
			useItem:       936, // USE_S1_
		},
		weaponTemplate: registry.GetItemTemplateRegistry().Get,
	}
}

// NewSpiritShotHandler builds the ItemHandler for the "SpiritShot" item handler name.
func NewSpiritShotHandler(charged *registry.ChargedShotRegistry, notifier ShotEffectNotifier) ItemHandler {
	return &shotHandler{
		shot:           registry.ShotSpiritshot,
		charged:        charged,
		notifier:       notifier,
		msgs:           spiritShotMessages(),
		weaponTemplate: registry.GetItemTemplateRegistry().Get,
	}
}

// NewBlessedSpiritShotHandler builds the ItemHandler for the "BlessedSpiritShot"
// item handler name. Mechanically identical to a spiritshot (same weapon count,
// grade check and system messages — the blessed shot items even declare
// default_action="SPIRITSHOT"), but it holds a *separate* weapon charge
// (ShotBlessedSpiritshot) so it can be charged over a regular spiritshot,
// mirroring L2J's BlessedSpiritShot.java.
func NewBlessedSpiritShotHandler(charged *registry.ChargedShotRegistry, notifier ShotEffectNotifier) ItemHandler {
	return &shotHandler{
		shot:           registry.ShotBlessedSpiritshot,
		charged:        charged,
		notifier:       notifier,
		msgs:           spiritShotMessages(),
		weaponTemplate: registry.GetItemTemplateRegistry().Get,
	}
}

// spiritShotMessages returns the SystemMessage ids shared by the regular and
// blessed spiritshot handlers (L2J uses the same SPIRITSHOT messages for both).
func spiritShotMessages() shotMessages {
	return shotMessages{
		cannotUse:     532, // CANNOT_USE_SPIRITSHOTS
		gradeMismatch: 530, // SPIRITSHOTS_GRADE_MISMATCH
		notEnough:     531, // NOT_ENOUGH_SPIRITSHOTS
		enabled:       533, // ENABLED_SPIRITSHOT
		useItem:       936, // USE_S1_
	}
}

// weaponShotCount returns how many shots this weapon consumes per charge for the
// handler's shot type (L2J L2Weapon.getSoulShotCount / getSpiritShotCount).
func (h *shotHandler) weaponShotCount(weapon *registry.ItemTemplate) int {
	if weapon == nil {
		return 0
	}
	// Spiritshots and blessed spiritshots both draw on the weapon's spiritshot
	// count (L2J L2Weapon.getSpiritShotCount serves both).
	if h.shot == registry.ShotSpiritshot || h.shot == registry.ShotBlessedSpiritshot {
		return weapon.Spiritshots
	}
	return weapon.Soulshots
}

// UseItem implements ItemHandler.
func (h *shotHandler) UseItem(ctx context.Context, use ItemUseContext) (bool, error) {
	// 1. Resolve the active weapon (right hand; two-handed weapons live there too).
	weapon, err := use.Repo.Item().GetEquippedItem(ctx, use.CharID, models.SlotRHand)
	if err != nil {
		return false, err
	}

	var weaponTmpl *registry.ItemTemplate
	if weapon != nil {
		weaponTmpl = h.weaponTemplate(weapon.ItemID)
	}
	shotCount := h.weaponShotCount(weaponTmpl)

	// No weapon, or a weapon that cannot use this shot type. Auto recharges stay
	// silent — a failure message every swing would spam the owner.
	if weapon == nil || shotCount == 0 {
		if !use.Auto {
			h.notify(func(n ShotEffectNotifier) { n.SystemMessage(use.CharID, h.msgs.cannotUse) })
		}
		return false, nil
	}

	// 2. Grade check: weapon crystal grade must match the shot grade (S/S80/S84
	//    collapse to a single "S+" grade, exactly like L2J getItemGradeSPlus).
	if gradeSPlus(weaponTmpl.CrystalType) != gradeSPlus(use.Template.CrystalType) {
		if !use.Auto {
			h.notify(func(n ShotEffectNotifier) { n.SystemMessage(use.CharID, h.msgs.gradeMismatch) })
		}
		return false, nil
	}

	// 3. Already charged: nothing to do, do not consume again.
	if h.charged.IsCharged(weapon.ObjectID, h.shot) {
		return false, nil
	}

	// 4. Consume the shots from the used stack.
	if use.Item.Count < int64(shotCount) {
		if !use.Auto {
			h.notify(func(n ShotEffectNotifier) { n.SystemMessage(use.CharID, h.msgs.notEnough) })
		}
		return false, nil
	}

	remaining := use.Item.Count - int64(shotCount)
	if remaining <= 0 {
		if err := use.Repo.Item().Delete(ctx, use.Item.ObjectID); err != nil {
			return false, err
		}
		use.Item.Count = 0
	} else {
		use.Item.Count = remaining
		if err := use.Repo.Item().Update(ctx, use.Item); err != nil {
			return false, err
		}
	}

	// 5. Charge the weapon instance, recording the weapon grade for the hit visual.
	h.charged.Charge(weapon.ObjectID, h.shot, int(gradeSPlus(weaponTmpl.CrystalType)))

	// 6. Client feedback + activation visual. An auto recharge skips the chat
	//    messages (they would spam every swing) but keeps the shot visual.
	skillID, skillLevel := shotVisualSkill(use.Template)
	h.notify(func(n ShotEffectNotifier) {
		if !use.Auto {
			n.ItemSystemMessage(use.CharID, h.msgs.useItem, use.Template.ID)
			n.SystemMessage(use.CharID, h.msgs.enabled)
		}
		if skillID > 0 {
			n.BroadcastShotVisual(use.CharID, skillID, skillLevel)
		}
	})

	log.Ctx(ctx).Debug().
		Int32("char_id", use.CharID).
		Int32("weapon_object_id", weapon.ObjectID).
		Int32("shot_item_id", use.Template.ID).
		Int("consumed", shotCount).
		Msg("shot charged")

	return true, nil
}

func (h *shotHandler) notify(fn func(ShotEffectNotifier)) {
	if h.notifier != nil {
		fn(h.notifier)
	}
}

// shotVisualSkill returns the skill id/level used for the MagicSkillUse
// activation animation, taken from the shot item's linked skill (item_skill).
func shotVisualSkill(shot *registry.ItemTemplate) (int32, int32) {
	if shot == nil || len(shot.ItemSkills) == 0 {
		return 0, 0
	}
	s := shot.ItemSkills[0]
	return int32(s.ID), int32(s.Level)
}

// gradeSPlus collapses the S / S80 / S84 grades into a single grade, mirroring
// L2J's L2Item.getItemGradeSPlus(): those three grades share soul/spirit shots.
func gradeSPlus(g registry.ItemGrade) registry.ItemGrade {
	switch g {
	case registry.GradeS80, registry.GradeS84:
		return registry.GradeS
	default:
		return g
	}
}
