package usecase

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// EnchantResultCode mirrors the first field of L2J's EnchantResult packet
// (serverpackets/EnchantResult.java + clientpackets/RequestEnchantItem.java).
type EnchantResultCode int32

const (
	// EnchantCodeSuccess: enchant succeeded, item enchant level +1.
	EnchantCodeSuccess EnchantResultCode = 0
	// EnchantCodeFailCrystal: enchant failed, item destroyed, crystals returned.
	EnchantCodeFailCrystal EnchantResultCode = 1
	// EnchantCodeError: invalid conditions, nothing consumed.
	EnchantCodeError EnchantResultCode = 2
	// EnchantCodeBlessedFail: blessed scroll failure, enchant reset to 0.
	EnchantCodeBlessedFail EnchantResultCode = 3
	// EnchantCodeFailDestroy: enchant failed, item destroyed, no crystals.
	EnchantCodeFailDestroy EnchantResultCode = 4
	// EnchantCodeSafeFail: safe scroll failure, enchant level unchanged.
	EnchantCodeSafeFail EnchantResultCode = 5
)

// System message ids used by the enchant flow (L2J SystemMessageId.java).
const (
	sysMsgInappropriateEnchant int32 = 355  // INAPPROPRIATE_ENCHANT_CONDITION
	sysMsgBlessedEnchantFailed int32 = 1517 // BLESSED_ENCHANT_FAILED
	sysMsgSafeEnchantFailed    int32 = 6004  // SAFE_ENCHANT_FAILED
	sysMsgEnchantInProgress    int32 = 2188  // ENCHANTMENT_ALREADY_IN_PROGRESS
	sysMsgDoesNotFitScroll     int32 = 424   // DOES_NOT_FIT_SCROLL_CONDITIONS
)

// enchantScrollInfo is the static profile of an enchant scroll, combining the
// classification derived from the item's etcitem_type (weapon/blessed/safe) with
// the tuning loaded from enchantItemData.xml (target grade, caps, bonus rate).
type enchantScrollInfo struct {
	isWeapon    bool
	isBlessed   bool
	isSafe      bool
	targetGrade registry.ItemGrade // collapsed base grade (D/C/B/A/S)
	maxEnchant  int                // 0 or 65535 => effectively unlimited
	bonusRate   float64            // additive success chance from the scroll
}

// enchantTarget is the static+dynamic profile of the item being enchanted.
// baseChance is the success chance resolved from the enchant-groups data for the
// scroll group / target / current enchant level (before the scroll's bonus).
type enchantTarget struct {
	type2        registry.ItemType2
	grade        registry.ItemGrade // already collapsed via gradeSPlus
	enchantLevel int
	enchantable  bool
	baseChance   float64
}

// enchantDecision is the pure outcome of an enchant attempt: what result code to
// report to the client and how the item/scroll state must change. It is produced
// by decideEnchant so the rules stay unit-testable without repo/packet deps.
type enchantDecision struct {
	code          EnchantResultCode
	newLevel      int   // target enchant level after the attempt
	consumeScroll bool  // scroll consumed (true for every non-error outcome)
	destroyTarget bool  // target item destroyed (normal-scroll failure)
	systemMsg     int32 // optional SystemMessage id to send (0 => none)
}

// decideEnchant applies the L2J High Five enchant rules. roll is a uniformly
// distributed value in [0,100): success when roll < finalChance. This mirrors
// EnchantScroll.calculateSuccess + RequestEnchantItem.runImpl.
func decideEnchant(s enchantScrollInfo, t enchantTarget, roll float64) enchantDecision {
	// --- validation (matches AbstractEnchantItem.isValid); nothing is consumed ---
	if !validateEnchant(s, t) {
		return enchantDecision{code: EnchantCodeError, newLevel: t.enchantLevel, systemMsg: sysMsgInappropriateEnchant}
	}

	// Valid target: the scroll is spent regardless of success/failure. The base
	// chance is resolved from the enchant-groups data by the caller.
	finalChance := math.Min(t.baseChance+s.bonusRate, 100)
	if roll < finalChance {
		return enchantDecision{code: EnchantCodeSuccess, newLevel: t.enchantLevel + 1, consumeScroll: true}
	}

	// Failure branch depends on the scroll type.
	switch {
	case s.isSafe:
		// Safe enchant: enchant level remains, item preserved.
		return enchantDecision{code: EnchantCodeSafeFail, newLevel: t.enchantLevel, consumeScroll: true, systemMsg: sysMsgSafeEnchantFailed}
	case s.isBlessed:
		// Blessed enchant: item preserved but enchant level reset to 0.
		return enchantDecision{code: EnchantCodeBlessedFail, newLevel: 0, consumeScroll: true, systemMsg: sysMsgBlessedEnchantFailed}
	default:
		// Regular enchant: item is destroyed. Crystal reward (code 1) is parked;
		// we report a plain destruction (code 4).
		return enchantDecision{code: EnchantCodeFailDestroy, newLevel: 0, consumeScroll: true, destroyTarget: true}
	}
}

// validateEnchant reports whether the scroll may be applied to the target,
// mirroring AbstractEnchantItem.isValid: the item must be enchantable, its type2
// must match the scroll's weapon/armor kind, its collapsed grade must equal the
// scroll's target grade, and its enchant level must be within the scroll's cap
// (a cap of 0 means "no explicit cap" in enchantItemData.xml).
func validateEnchant(s enchantScrollInfo, t enchantTarget) bool {
	if !t.enchantable || !enchantTypeMatches(s.isWeapon, t.type2) || t.grade != s.targetGrade {
		return false
	}
	if s.maxEnchant > 0 && t.enchantLevel > s.maxEnchant {
		return false
	}
	return true
}

// enchantTypeMatches reports whether a weapon/armor scroll can target the given
// item type2 (weapon scrolls -> weapons; armor scrolls -> armor/accessories).
func enchantTypeMatches(scrollIsWeapon bool, type2 registry.ItemType2) bool {
	if scrollIsWeapon {
		return type2 == registry.ItemType2Weapon
	}
	return type2 == registry.ItemType2Armor || type2 == registry.ItemType2Accessory
}

// EnchantChanceSource resolves the retail success chance for a scroll group /
// target / enchant level from the parsed enchantItemGroups.xml data. Implemented
// by registry.EnchantGroupsRegistry; abstracted here for testability.
type EnchantChanceSource interface {
	Chance(scrollGroupID int, bodyPart int32, isMagicWeapon bool, itemID int32, enchantLevel int) (float64, bool)
}

// classifyEnchantScroll reports whether the etcitem_type denotes an enchant
// scroll and, if so, its weapon/blessed/safe flags. Mirrors EnchantScroll's
// constructor classification.
func classifyEnchantScroll(etc registry.EtcItemType) (ok, isWeapon, isBlessed, isSafe bool) {
	s := string(etc)
	switch s {
	case "SCRL_ENCHANT_WP", "SCRL_ENCHANT_AM",
		"BLESS_SCRL_ENCHANT_WP", "BLESS_SCRL_ENCHANT_AM",
		"ANCIENT_CRYSTAL_ENCHANT_WP", "ANCIENT_CRYSTAL_ENCHANT_AM",
		"SCRL_INC_ENCHANT_PROP_WP", "SCRL_INC_ENCHANT_PROP_AM":
		ok = true
	default:
		return false, false, false, false
	}
	isWeapon = strings.HasSuffix(s, "_WP")
	isBlessed = strings.HasPrefix(s, "BLESS_")
	isSafe = strings.HasPrefix(s, "ANCIENT_CRYSTAL_")
	return ok, isWeapon, isBlessed, isSafe
}

// EnchantDataSource resolves an enchant scroll item id to its tuning data
// (target grade / caps / bonus rate) loaded from enchantItemData.xml.
type EnchantDataSource interface {
	GetEnchantScroll(itemID int32) (registry.EnchantScrollData, bool)
}

// EnchantNotifier delivers the "pick a target" prompt (ChooseInventoryItem) to a
// scroll's owner. It decouples the item handler from the network/world layer,
// exactly like ShotEffectNotifier. A nil notifier makes the prompt a no-op.
type EnchantNotifier interface {
	ChooseInventoryItem(charID int32, itemID int32)
	SystemMessage(charID int32, msgID int32)
}

// EnchantUseCase implements the two-step enchant flow: an item handler that arms
// a scroll (UseItem -> ChooseInventoryItem) and EnchantItem which performs the
// actual enchant when the client answers with RequestEnchantItem.
type EnchantUseCase struct {
	repo     repo.DatabaseRepository
	data     EnchantDataSource
	chances  EnchantChanceSource
	state    *registry.EnchantStateRegistry
	notifier EnchantNotifier
	// itemTemplate resolves static templates; overridable in tests.
	itemTemplate func(itemID int32) *registry.ItemTemplate
	// rng returns a value in [0,1); injected for deterministic tests.
	rng func() float64
}

// NewEnchantUseCase wires an enchant use case. rng may be nil (defaults to the
// global math/rand source). chances supplies the retail success chances parsed
// from enchantItemGroups.xml.
func NewEnchantUseCase(db repo.DatabaseRepository, data EnchantDataSource, chances EnchantChanceSource, state *registry.EnchantStateRegistry, notifier EnchantNotifier, rng func() float64) *EnchantUseCase {
	if rng == nil {
		rng = rand.Float64
	}
	return &EnchantUseCase{
		repo:         db,
		data:         data,
		chances:      chances,
		state:        state,
		notifier:     notifier,
		itemTemplate: registry.GetItemTemplateRegistry().Get,
		rng:          rng,
	}
}

// ScrollHandler returns the ItemHandler registered under "EnchantScrolls": on
// use it arms the scroll and prompts the client to pick a target.
func (uc *EnchantUseCase) ScrollHandler() ItemHandler { return &enchantScrollHandler{uc: uc} }

// enchantScrollHandler implements ItemHandler for enchant scrolls. It never
// consumes the scroll (consumed=false): the scroll is only spent later in
// EnchantItem, mirroring L2J's datapack EnchantScrolls handler.
type enchantScrollHandler struct{ uc *EnchantUseCase }

func (h *enchantScrollHandler) UseItem(ctx context.Context, use ItemUseContext) (bool, error) {
	if use.Template == nil {
		return false, nil
	}
	ok, _, _, _ := classifyEnchantScroll(use.Template.EtcItemType)
	if !ok {
		// Not actually an enchant scroll; behave like an unhandled item.
		return false, nil
	}

	// Already arming/enchanting: refuse a second scroll (L2J isEnchanting()).
	if h.uc.state.HasActive(use.CharID) {
		h.uc.notify(func(n EnchantNotifier) { n.SystemMessage(use.CharID, sysMsgEnchantInProgress) })
		return false, nil
	}

	h.uc.state.SetActive(use.CharID, registry.ActiveEnchant{
		ScrollObjectID: use.Item.ObjectID,
		ScrollItemID:   use.Template.ID,
	})
	h.uc.notify(func(n EnchantNotifier) { n.ChooseInventoryItem(use.CharID, use.Template.ID) })

	log.Ctx(ctx).Debug().
		Int32("char_id", use.CharID).
		Int32("scroll_object_id", use.Item.ObjectID).
		Int32("scroll_item_id", use.Template.ID).
		Msg("enchant scroll armed, awaiting target")

	// consumed=false: no InventoryUpdate, scroll not spent yet.
	return false, nil
}

func (uc *EnchantUseCase) notify(fn func(EnchantNotifier)) {
	if uc.notifier != nil {
		fn(uc.notifier)
	}
}

// EnchantOutcome describes the packets the client handler must emit after an
// enchant attempt.
type EnchantOutcome struct {
	Code         EnchantResultCode
	Crystal      int32
	CrystalCount int64
	SystemMsg    int32 // 0 => none

	// Item state after the attempt (for InventoryUpdate). Target is nil when the
	// item was destroyed; Scroll is nil when the last scroll was consumed.
	Target          *models.CharacterItem
	TargetDestroyed bool
	Scroll          *models.CharacterItem
	ScrollRemoved   bool
	NewEnchantLevel int
	Success         bool
}

// PutEnchantResult classifies the outcome of a RequestExTryToPutEnchantTargetItem
// (the High Five windowed flow, where the client drops a target into the open
// enchant window). It mirrors the three exit paths of L2J's
// RequestExTryToPutEnchantTargetItem.runImpl.
type PutEnchantResult int

const (
	// PutEnchantIgnore: no armed scroll / bad ids / item not owned. L2J returns
	// silently here — the caller must send nothing.
	PutEnchantIgnore PutEnchantResult = iota
	// PutEnchantInvalid: the target does not fit the scroll conditions. The caller
	// must reply ExPutEnchantTargetItemResult(0) plus a system message; the active
	// state has already been cleared.
	PutEnchantInvalid
	// PutEnchantAccepted: the target fits. The caller must reply
	// ExPutEnchantTargetItemResult(targetObjectID); the scroll stays armed until
	// RequestEnchantItem (or RequestExCancelEnchantItem) resolves it.
	PutEnchantAccepted
)

// ValidateTarget checks whether the item the player dropped into the enchant
// window fits the currently-armed scroll, without consuming or enchanting
// anything. It mirrors L2J's RequestExTryToPutEnchantTargetItem: on an invalid
// target it clears the player's active-enchant state (so the window must be
// re-opened) and returns the DOES_NOT_FIT_SCROLL_CONDITIONS message id; on a
// valid target it leaves the state armed for the subsequent RequestEnchantItem.
func (uc *EnchantUseCase) ValidateTarget(ctx context.Context, charID int32, targetObjectID int32) (PutEnchantResult, int32, error) {
	active, ok := uc.state.GetActive(charID)
	if !ok || targetObjectID == 0 {
		return PutEnchantIgnore, 0, nil
	}

	scroll, err := uc.repo.Item().GetByObjectID(ctx, active.ScrollObjectID)
	if err != nil {
		return PutEnchantIgnore, 0, fmt.Errorf("load scroll: %w", err)
	}
	target, err := uc.repo.Item().GetByObjectID(ctx, targetObjectID)
	if err != nil {
		return PutEnchantIgnore, 0, fmt.Errorf("load target: %w", err)
	}
	if scroll == nil || target == nil || scroll.OwnerID != charID || target.OwnerID != charID {
		// Stale/hostile request: the scroll or target is gone. Silent (L2J returns).
		return PutEnchantIgnore, 0, nil
	}

	scrollTmpl := uc.itemTemplate(scroll.ItemID)
	targetTmpl := uc.itemTemplate(target.ItemID)
	sd, found := uc.data.GetEnchantScroll(scroll.ItemID)
	if scrollTmpl == nil || targetTmpl == nil || !found {
		uc.state.Clear(charID)
		return PutEnchantInvalid, sysMsgDoesNotFitScroll, nil
	}
	okScroll, isWeapon, isBlessed, isSafe := classifyEnchantScroll(scrollTmpl.EtcItemType)
	if !okScroll {
		uc.state.Clear(charID)
		return PutEnchantInvalid, sysMsgDoesNotFitScroll, nil
	}

	sInfo := enchantScrollInfo{
		isWeapon:    isWeapon,
		isBlessed:   isBlessed,
		isSafe:      isSafe,
		targetGrade: sd.TargetGrade,
		maxEnchant:  sd.MaxEnchant,
		bonusRate:   sd.BonusRate,
	}
	tInfo := enchantTarget{
		type2:        targetTmpl.Type2,
		grade:        gradeSPlus(targetTmpl.CrystalType),
		enchantLevel: target.EnchantLevel,
		enchantable:  targetTmpl.Enchantable,
	}
	if !validateEnchant(sInfo, tInfo) {
		uc.state.Clear(charID)
		return PutEnchantInvalid, sysMsgDoesNotFitScroll, nil
	}

	log.Ctx(ctx).Debug().
		Int32("char_id", charID).
		Int32("target_object_id", targetObjectID).
		Int32("scroll_item_id", scroll.ItemID).
		Msg("enchant target accepted into window")
	return PutEnchantAccepted, 0, nil
}

// CancelEnchant clears any active-enchant arming for a character, closing the
// enchant window. Mirrors L2J's RequestExCancelEnchantItem (which also emits an
// EnchantResult(2) handled by the caller).
func (uc *EnchantUseCase) CancelEnchant(charID int32) {
	uc.state.Clear(charID)
}

// EnchantItem performs the actual enchant when the client answers a prompt with
// RequestEnchantItem. It clears the player's active-enchant state, applies the
// item/scroll changes to the repository, and returns the outcome to be sent.
// Returns (nil, nil) when there is nothing to do (no active scroll / bad ids).
func (uc *EnchantUseCase) EnchantItem(ctx context.Context, charID int32, targetObjectID int32) (*EnchantOutcome, error) {
	active, ok := uc.state.GetActive(charID)
	// The active state is always cleared once we start processing (matches L2J's
	// setActiveEnchantItemId(ID_NONE) on every exit path).
	uc.state.Clear(charID)
	if !ok || targetObjectID == 0 {
		return nil, nil
	}

	scroll, err := uc.repo.Item().GetByObjectID(ctx, active.ScrollObjectID)
	if err != nil {
		return nil, fmt.Errorf("load scroll: %w", err)
	}
	target, err := uc.repo.Item().GetByObjectID(ctx, targetObjectID)
	if err != nil {
		return nil, fmt.Errorf("load target: %w", err)
	}
	if scroll == nil || target == nil || target.OwnerID != charID || scroll.OwnerID != charID {
		return &EnchantOutcome{Code: EnchantCodeError, SystemMsg: sysMsgInappropriateEnchant}, nil
	}

	scrollTmpl := uc.itemTemplate(scroll.ItemID)
	targetTmpl := uc.itemTemplate(target.ItemID)
	if scrollTmpl == nil || targetTmpl == nil {
		return &EnchantOutcome{Code: EnchantCodeError, SystemMsg: sysMsgInappropriateEnchant}, nil
	}

	okScroll, isWeapon, isBlessed, isSafe := classifyEnchantScroll(scrollTmpl.EtcItemType)
	if !okScroll {
		return &EnchantOutcome{Code: EnchantCodeError, SystemMsg: sysMsgInappropriateEnchant}, nil
	}

	// Scroll tuning (targetGrade / maxEnchant / bonusRate) from enchant data.
	sd, found := uc.data.GetEnchantScroll(scroll.ItemID)
	if !found {
		return &EnchantOutcome{Code: EnchantCodeError, SystemMsg: sysMsgInappropriateEnchant}, nil
	}

	sInfo := enchantScrollInfo{
		isWeapon:    isWeapon,
		isBlessed:   isBlessed,
		isSafe:      isSafe,
		targetGrade: sd.TargetGrade,
		maxEnchant:  sd.MaxEnchant,
		bonusRate:   sd.BonusRate,
	}

	// Retail success chance from the scroll's group and the target's slot / magic
	// flag / current enchant level. A missing binding (L2J getChance == -1) is an
	// invalid enchant condition -> nothing consumed.
	baseChance, chanceOK := uc.chances.Chance(
		sd.ScrollGroupID, targetTmpl.BodyPartCode, targetTmpl.IsMagicWeapon, target.ItemID, target.EnchantLevel,
	)
	if !chanceOK {
		return &EnchantOutcome{Code: EnchantCodeError, SystemMsg: sysMsgInappropriateEnchant}, nil
	}

	tInfo := enchantTarget{
		type2:        targetTmpl.Type2,
		grade:        gradeSPlus(targetTmpl.CrystalType),
		enchantLevel: target.EnchantLevel,
		enchantable:  targetTmpl.Enchantable,
		baseChance:   baseChance,
	}

	decision := decideEnchant(sInfo, tInfo, 100*uc.rng())

	outcome := &EnchantOutcome{
		Code:            decision.code,
		SystemMsg:       decision.systemMsg,
		NewEnchantLevel: decision.newLevel,
		Success:         decision.code == EnchantCodeSuccess,
	}

	// Error: nothing consumed, nothing changed.
	if decision.code == EnchantCodeError {
		return outcome, nil
	}

	// Consume one scroll. The scroll pointer is always returned so the caller can
	// emit an InventoryUpdate (modify with the new count, or remove when depleted).
	if err := uc.consumeScroll(ctx, scroll); err != nil {
		return nil, err
	}
	outcome.Scroll = scroll
	outcome.ScrollRemoved = scroll.Count <= 0

	// Apply the target change. The target pointer is always returned (even on
	// destruction) so the caller can build the InventoryUpdate remove entry.
	outcome.Target = target
	if decision.destroyTarget {
		if err := uc.repo.Item().Delete(ctx, target.ObjectID); err != nil {
			return nil, fmt.Errorf("destroy target: %w", err)
		}
		outcome.TargetDestroyed = true
	} else {
		target.EnchantLevel = decision.newLevel
		if err := uc.repo.Item().Update(ctx, target); err != nil {
			return nil, fmt.Errorf("update target enchant: %w", err)
		}
	}

	log.Ctx(ctx).Info().
		Int32("char_id", charID).
		Int32("target_object_id", target.ObjectID).
		Int32("scroll_item_id", scroll.ItemID).
		Int("code", int(decision.code)).
		Int("new_enchant", decision.newLevel).
		Msg("enchant attempt resolved")

	return outcome, nil
}

// consumeScroll removes a single scroll unit, deleting the row on the last unit.
func (uc *EnchantUseCase) consumeScroll(ctx context.Context, scroll *models.CharacterItem) error {
	scroll.Count--
	if scroll.Count <= 0 {
		scroll.Count = 0
		if err := uc.repo.Item().Delete(ctx, scroll.ObjectID); err != nil {
			return fmt.Errorf("delete scroll: %w", err)
		}
		return nil
	}
	if err := uc.repo.Item().Update(ctx, scroll); err != nil {
		return fmt.Errorf("update scroll count: %w", err)
	}
	return nil
}
