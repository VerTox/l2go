package usecase

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// blessedShot is a no-grade blessed spiritshot template (default_action
// SPIRITSHOT, linked skill 2061-1), mirroring item 3947.
func blessedShot() *registry.ItemTemplate {
	return &registry.ItemTemplate{
		ID:          3947,
		Name:        "Blessed Spiritshot: No Grade",
		Handler:     "BlessedSpiritShot",
		CrystalType: registry.GradeNone,
		ItemSkills:  []registry.ItemSkill{{ID: 2061, Level: 1}},
	}
}

// spiritWeapon is a no-grade weapon that accepts spiritshots.
func spiritWeapon() *registry.ItemTemplate {
	return &registry.ItemTemplate{ID: weaponItemID, Name: "Staff", CrystalType: registry.GradeNone, Spiritshots: 1}
}

// newBlessedTestHandler builds a blessed-spiritshot handler wired to fakes.
func newBlessedTestHandler(charged *registry.ChargedShotRegistry, notifier ShotEffectNotifier, weaponTmpl *registry.ItemTemplate) *shotHandler {
	h := NewBlessedSpiritShotHandler(charged, notifier).(*shotHandler)
	h.weaponTemplate = func(int32) *registry.ItemTemplate { return weaponTmpl }
	return h
}

func TestBlessedSpiritShot_Success_ChargesBlessedAndConsumes(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{weapon: &models.CharacterItem{ObjectID: weaponObjID, ItemID: weaponItemID}}
	h := newBlessedTestHandler(charged, notifier, spiritWeapon())

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 3947, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: blessedShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true on successful blessed charge")
	}
	if !charged.IsCharged(weaponObjID, registry.ShotBlessedSpiritshot) {
		t.Error("weapon not charged with blessed spiritshot after successful use")
	}
	// Blessed must NOT set the plain spiritshot charge.
	if charged.IsCharged(weaponObjID, registry.ShotSpiritshot) {
		t.Error("blessed shot wrongly set the plain spiritshot charge")
	}
	if repoFake.updateCalls != 1 || repoFake.updated.Count != 99 {
		t.Errorf("consume wrong: updateCalls=%d count=%d, want 1 / 99", repoFake.updateCalls, repoFake.updated.Count)
	}
	if notifier.visualCall != 1 || notifier.visualID != 2061 || notifier.visualLvl != 1 {
		t.Errorf("visual = (calls=%d id=%d lvl=%d), want (1, 2061, 1)", notifier.visualCall, notifier.visualID, notifier.visualLvl)
	}
	if !notifier.sawSysMsg(533) { // ENABLED_SPIRITSHOT
		t.Errorf("expected ENABLED_SPIRITSHOT (533), got %v", notifier.sysMsgs)
	}
}

// A blessed spiritshot can be charged even while a regular spiritshot charge is
// already present (separate ShotType), mirroring L2J's "can be charged over
// SpiritShot".
func TestBlessedSpiritShot_ChargesOverPlainSpiritshot(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	charged.SetCharged(weaponObjID, registry.ShotSpiritshot, true) // plain already charged
	repoFake := &shotFakeItemRepo{weapon: &models.CharacterItem{ObjectID: weaponObjID, ItemID: weaponItemID}}
	h := newBlessedTestHandler(charged, nil, spiritWeapon())

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 3947, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: blessedShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true: blessed should charge over plain spiritshot")
	}
	if !charged.IsCharged(weaponObjID, registry.ShotBlessedSpiritshot) {
		t.Error("blessed charge not set")
	}
	if repoFake.updateCalls != 1 {
		t.Errorf("updateCalls = %d, want 1", repoFake.updateCalls)
	}
}

func TestBlessedSpiritShot_AlreadyCharged_NoDoubleConsume(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	charged.SetCharged(weaponObjID, registry.ShotBlessedSpiritshot, true) // pre-charged blessed
	repoFake := &shotFakeItemRepo{weapon: &models.CharacterItem{ObjectID: weaponObjID, ItemID: weaponItemID}}
	h := newBlessedTestHandler(charged, nil, spiritWeapon())

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 3947, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: blessedShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false when blessed already charged")
	}
	if repoFake.updateCalls != 0 || repoFake.deleteCalls != 0 {
		t.Error("item consumed while blessed already charged, want no re-consumption")
	}
}

func TestBlessedSpiritShot_GradeMismatch_NoConsume(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{weapon: &models.CharacterItem{ObjectID: weaponObjID, ItemID: weaponItemID}}
	// D-grade weapon, no-grade blessed shot → mismatch.
	dWeapon := &registry.ItemTemplate{ID: weaponItemID, CrystalType: registry.GradeD, Spiritshots: 1}
	h := newBlessedTestHandler(charged, notifier, dWeapon)

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 3947, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: blessedShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false on grade mismatch")
	}
	if charged.IsCharged(weaponObjID, registry.ShotBlessedSpiritshot) {
		t.Error("weapon charged on grade mismatch")
	}
	if !notifier.sawSysMsg(530) { // SPIRITSHOTS_GRADE_MISMATCH
		t.Errorf("expected SPIRITSHOTS_GRADE_MISMATCH (530), got %v", notifier.sysMsgs)
	}
}

func TestBlessedSpiritShot_NoWeapon_CannotUse(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{weapon: nil}
	h := newBlessedTestHandler(charged, notifier, nil)

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 3947, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: blessedShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false with no weapon")
	}
	if !notifier.sawSysMsg(532) { // CANNOT_USE_SPIRITSHOTS
		t.Errorf("expected CANNOT_USE_SPIRITSHOTS (532), got %v", notifier.sysMsgs)
	}
}

// --- Beast (parked) ---

func TestBeastShot_NoSummon_NoConsume(t *testing.T) {
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{}
	h := NewBeastShotHandler(notifier)

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 6645, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: &registry.ItemTemplate{ID: 6645, Handler: "BeastSoulShot"}, Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false (no pet system)")
	}
	if repoFake.updateCalls != 0 || repoFake.deleteCalls != 0 {
		t.Errorf("beast shot consumed an item (update=%d delete=%d), want none", repoFake.updateCalls, repoFake.deleteCalls)
	}
	if !notifier.sawSysMsg(574) { // PETS_ARE_NOT_AVAILABLE_AT_THIS_TIME
		t.Errorf("expected PETS_ARE_NOT_AVAILABLE_AT_THIS_TIME (574), got %v", notifier.sysMsgs)
	}
}

func TestBeastShot_NilNotifier_DoesNotPanic(t *testing.T) {
	repoFake := &shotFakeItemRepo{}
	h := NewBeastShotHandler(nil)
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: &models.CharacterItem{ObjectID: shotObjID, ItemID: 6645, Count: 100},
		Template: &registry.ItemTemplate{ID: 6645, Handler: "BeastSpiritShot"}, Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil || consumed {
		t.Fatalf("want (false, nil), got (%v, %v)", consumed, err)
	}
}

// --- Fish (parked) ---

func TestFishShot_NoFishing_SilentNoConsume(t *testing.T) {
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{}
	h := NewFishShotHandler()

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 6535, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: &registry.ItemTemplate{ID: 6535, Handler: "FishShots"}, Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false (no fishing system)")
	}
	if repoFake.updateCalls != 0 || repoFake.deleteCalls != 0 {
		t.Errorf("fish shot consumed an item (update=%d delete=%d), want none", repoFake.updateCalls, repoFake.deleteCalls)
	}
	// L2J's no-fishing-rod branch is silent — no system message expected.
	if len(notifier.sysMsgs) != 0 {
		t.Errorf("fish shot sent system messages %v, want none", notifier.sysMsgs)
	}
}
