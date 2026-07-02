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
type enchantTarget struct {
	type2         registry.ItemType2
	grade         registry.ItemGrade // already collapsed via gradeSPlus
	isMagicWeapon bool
	isFullArmor   bool
	enchantLevel  int
	enchantable   bool
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
	if !t.enchantable || !enchantTypeMatches(s.isWeapon, t.type2) ||
		t.grade != s.targetGrade ||
		(s.maxEnchant > 0 && t.enchantLevel > s.maxEnchant) {
		return enchantDecision{code: EnchantCodeError, newLevel: t.enchantLevel, systemMsg: sysMsgInappropriateEnchant}
	}

	// Valid target: the scroll is spent regardless of success/failure.
	finalChance := math.Min(enchantBaseChance(t)+s.bonusRate, 100)
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

// enchantTypeMatches reports whether a weapon/armor scroll can target the given
// item type2 (weapon scrolls -> weapons; armor scrolls -> armor/accessories).
func enchantTypeMatches(scrollIsWeapon bool, type2 registry.ItemType2) bool {
	if scrollIsWeapon {
		return type2 == registry.ItemType2Weapon
	}
	return type2 == registry.ItemType2Armor || type2 == registry.ItemType2Accessory
}

// rangeChance is one row of an enchant chance table: [min,max] enchant level ->
// success chance in percent.
type rangeChance struct {
	min, max int
	chance   float64
}

// Chance tables transcribed from data/enchantItemGroups.xml (HF default scroll
// group id=0). Exact per-slot bindings and non-default scroll groups are parked;
// this covers the four base groups the default scrolls bind to.
var (
	armorGroup = []rangeChance{
		{0, 2, 100}, {3, 3, 66.67}, {4, 4, 33.34}, {5, 5, 25}, {6, 6, 20},
		{7, 7, 16.67}, {8, 8, 14.29}, {9, 9, 12.5}, {10, 10, 11.12}, {11, 11, 10.0},
		{12, 12, 9.10}, {13, 13, 8.34}, {14, 14, 7.70}, {15, 15, 7.15}, {16, 16, 6.67},
		{17, 17, 6.25}, {18, 18, 5.89}, {19, 19, 5.56}, {20, 65535, 0},
	}
	fullArmorGroup = []rangeChance{
		{0, 3, 100}, {4, 4, 66.67}, {5, 5, 33.34}, {6, 6, 25}, {7, 7, 20},
		{8, 8, 16.67}, {9, 9, 14.29}, {10, 10, 12.5}, {11, 11, 11.12}, {12, 12, 10.0},
		{13, 13, 9.10}, {14, 14, 8.34}, {15, 15, 7.70}, {16, 16, 7.15}, {17, 17, 6.67},
		{18, 18, 6.25}, {19, 19, 5.89}, {20, 65535, 0},
	}
	fighterWeaponGroup = []rangeChance{{0, 2, 100}, {3, 14, 70}, {15, 65535, 35}}
	mageWeaponGroup    = []rangeChance{{0, 2, 100}, {3, 14, 40}, {15, 65535, 20}}
)

// enchantBaseChance returns the base success chance for the target at its current
// enchant level, selecting the chance group the way enchantScrollGroup id=0 binds.
func enchantBaseChance(t enchantTarget) float64 {
	if t.type2 == registry.ItemType2Weapon {
		if t.isMagicWeapon {
			return chanceFromGroup(mageWeaponGroup, t.enchantLevel)
		}
		return chanceFromGroup(fighterWeaponGroup, t.enchantLevel)
	}
	if t.isFullArmor {
		return chanceFromGroup(fullArmorGroup, t.enchantLevel)
	}
	return chanceFromGroup(armorGroup, t.enchantLevel)
}

func chanceFromGroup(group []rangeChance, level int) float64 {
	for _, r := range group {
		if level >= r.min && level <= r.max {
			return r.chance
		}
	}
	// Above all ranges: no chance (mirrors the 20-65535 => 0 tail).
	return 0
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
	state    *registry.EnchantStateRegistry
	notifier EnchantNotifier
	// itemTemplate resolves static templates; overridable in tests.
	itemTemplate func(itemID int32) *registry.ItemTemplate
	// rng returns a value in [0,1); injected for deterministic tests.
	rng func() float64
}

// NewEnchantUseCase wires an enchant use case. rng may be nil (defaults to the
// global math/rand source).
func NewEnchantUseCase(db repo.DatabaseRepository, data EnchantDataSource, state *registry.EnchantStateRegistry, notifier EnchantNotifier, rng func() float64) *EnchantUseCase {
	if rng == nil {
		rng = rand.Float64
	}
	return &EnchantUseCase{
		repo:         db,
		data:         data,
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
	tInfo := enchantTarget{
		type2:         targetTmpl.Type2,
		grade:         gradeSPlus(targetTmpl.CrystalType),
		isMagicWeapon: targetTmpl.IsMagicWeapon,
		isFullArmor:   models.IsFullArmor(targetTmpl.BodyPartCode),
		enchantLevel:  target.EnchantLevel,
		enchantable:   targetTmpl.Enchantable,
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
