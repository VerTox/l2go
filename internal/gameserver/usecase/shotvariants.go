package usecase

import (
	"context"

	"github.com/rs/zerolog/log"
)

// PETS_ARE_NOT_AVAILABLE_AT_THIS_TIME — sent when a beast (summon) shot is used
// without an active servitor.
const msgPetsNotAvailable int32 = 574

// beastShotHandler is a PARKED handler for the "BeastSoulShot" / "BeastSpiritShot"
// item handler names. In L2J these charge the player's summon/servitor (not the
// player's weapon), spending the summon's soulshots/spiritshots-per-hit.
//
// This server has no pet/summon system yet, so a summon can never be present.
// The handler therefore always takes L2J's "no summon" branch: it informs the
// player that pets are unavailable and consumes nothing. Full beast-shot logic
// (charging the summon, consuming shots, the PET_USE_SPIRITSHOT visual) is
// parked on the future pet/summon subsystem.
type beastShotHandler struct {
	notifier ShotEffectNotifier // may be nil (silent)
}

// NewBeastShotHandler builds the parked handler for beast soul/spirit shots.
func NewBeastShotHandler(notifier ShotEffectNotifier) ItemHandler {
	return &beastShotHandler{notifier: notifier}
}

// UseItem implements ItemHandler. Always a no-op (no summon exists): it never
// consumes the item and returns consumed=false.
func (h *beastShotHandler) UseItem(ctx context.Context, use ItemUseContext) (bool, error) {
	if h.notifier != nil {
		h.notifier.SystemMessage(use.CharID, msgPetsNotAvailable)
	}
	log.Ctx(ctx).Debug().
		Int32("char_id", use.CharID).
		Int32("item_id", use.Template.ID).
		Msg("beast shot used without a summon (pet system not implemented) — no-op")
	return false, nil
}

// fishShotHandler is a PARKED handler for the "FishShots" item handler name.
// In L2J fishing shots charge an equipped fishing rod for the fishing minigame
// and consume one shot per cast. This server implements neither fishing rods as
// a weapon type nor the fishing system, so the handler mirrors L2J's "no fishing
// rod equipped" branch: a silent no-op that consumes nothing. Full fishing-shot
// logic is parked on the future fishing subsystem.
type fishShotHandler struct{}

// NewFishShotHandler builds the parked handler for fishing shots.
func NewFishShotHandler() ItemHandler { return &fishShotHandler{} }

// UseItem implements ItemHandler. Always a silent no-op (no fishing rod / fishing
// system): it never consumes the item and returns consumed=false.
func (h *fishShotHandler) UseItem(ctx context.Context, use ItemUseContext) (bool, error) {
	log.Ctx(ctx).Debug().
		Int32("char_id", use.CharID).
		Int32("item_id", use.Template.ID).
		Msg("fishing shot used but fishing system not implemented — no-op")
	return false, nil
}
