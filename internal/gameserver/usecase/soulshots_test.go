package usecase

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// shotFakeItemRepo implements repo.ItemRepository via embedding; only the
// methods the shot handler exercises are overridden.
type shotFakeItemRepo struct {
	repo.ItemRepository
	weapon      *models.CharacterItem
	weaponErr   error
	updated     *models.CharacterItem
	deletedID   int32
	updateCalls int
	deleteCalls int
}

func (f *shotFakeItemRepo) GetEquippedItem(_ context.Context, _ int32, _ models.PaperdollSlot) (*models.CharacterItem, error) {
	return f.weapon, f.weaponErr
}

func (f *shotFakeItemRepo) Update(_ context.Context, item *models.CharacterItem) error {
	f.updateCalls++
	f.updated = item
	return nil
}

func (f *shotFakeItemRepo) Delete(_ context.Context, objectID int32) error {
	f.deleteCalls++
	f.deletedID = objectID
	return nil
}

// shotFakeDB implements repo.DatabaseRepository via embedding; only Item() is used.
type shotFakeDB struct {
	repo.DatabaseRepository
	item repo.ItemRepository
}

func (f *shotFakeDB) Item() repo.ItemRepository { return f.item }

// recordingNotifier records the effects the handler requested.
type recordingNotifier struct {
	itemMsgs   []int32
	sysMsgs    []int32
	visualCall int
	visualID   int32
	visualLvl  int32
}

func (n *recordingNotifier) ItemSystemMessage(_ int32, msgID int32, _ int32) {
	n.itemMsgs = append(n.itemMsgs, msgID)
}
func (n *recordingNotifier) SystemMessage(_ int32, msgID int32) {
	n.sysMsgs = append(n.sysMsgs, msgID)
}
func (n *recordingNotifier) BroadcastShotVisual(_ int32, skillID int32, skillLevel int32) {
	n.visualCall++
	n.visualID = skillID
	n.visualLvl = skillLevel
}

func (n *recordingNotifier) sawSysMsg(id int32) bool {
	for _, m := range n.sysMsgs {
		if m == id {
			return true
		}
	}
	return false
}

const (
	weaponItemID = 1
	weaponObjID  = 5000
	shotObjID    = 6000
)

// newShotTestHandler builds a soulshot handler wired to the given fakes, with a
// weapon-template resolver returning the given weapon template.
func newShotTestHandler(charged *registry.ChargedShotRegistry, notifier ShotEffectNotifier, weaponTmpl *registry.ItemTemplate) *shotHandler {
	h := NewSoulShotHandler(charged, notifier).(*shotHandler)
	h.weaponTemplate = func(int32) *registry.ItemTemplate { return weaponTmpl }
	return h
}

// noGradeShot is a no-grade soulshot template (grade None, linked skill 2039-1).
func noGradeShot() *registry.ItemTemplate {
	return &registry.ItemTemplate{
		ID:          1835,
		Name:        "Soulshot: No Grade",
		Handler:     "SoulShots",
		CrystalType: registry.GradeNone,
		ItemSkills:  []registry.ItemSkill{{ID: 2039, Level: 1}},
	}
}

func noGradeWeapon() *registry.ItemTemplate {
	return &registry.ItemTemplate{ID: weaponItemID, Name: "Sword", CrystalType: registry.GradeNone, Soulshots: 1}
}

func TestShotHandler_NoWeapon_NoConsume(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{weapon: nil} // no weapon equipped
	h := newShotTestHandler(charged, notifier, nil)

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 1835, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: noGradeShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false when no weapon equipped")
	}
	if repoFake.updateCalls != 0 || repoFake.deleteCalls != 0 {
		t.Errorf("item was modified (update=%d delete=%d), want no consumption", repoFake.updateCalls, repoFake.deleteCalls)
	}
	if !notifier.sawSysMsg(339) { // CANNOT_USE_SOULSHOTS
		t.Errorf("expected CANNOT_USE_SOULSHOTS (339), got sysMsgs=%v", notifier.sysMsgs)
	}
}

func TestShotHandler_GradeMismatch_NoConsume(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{weapon: &models.CharacterItem{ObjectID: weaponObjID, ItemID: weaponItemID}}
	// Weapon is D-grade, shot is no-grade → mismatch.
	dWeapon := &registry.ItemTemplate{ID: weaponItemID, CrystalType: registry.GradeD, Soulshots: 1}
	h := newShotTestHandler(charged, notifier, dWeapon)

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 1835, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: noGradeShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false on grade mismatch")
	}
	if repoFake.updateCalls != 0 || repoFake.deleteCalls != 0 {
		t.Error("item consumed on grade mismatch, want no consumption")
	}
	if charged.IsCharged(weaponObjID, registry.ShotSoulshot) {
		t.Error("weapon charged on grade mismatch, want not charged")
	}
	if !notifier.sawSysMsg(337) { // SOULSHOTS_GRADE_MISMATCH
		t.Errorf("expected SOULSHOTS_GRADE_MISMATCH (337), got sysMsgs=%v", notifier.sysMsgs)
	}
}

func TestShotHandler_Success_ChargesAndConsumes(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{weapon: &models.CharacterItem{ObjectID: weaponObjID, ItemID: weaponItemID}}
	h := newShotTestHandler(charged, notifier, noGradeWeapon())

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 1835, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: noGradeShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true on successful charge")
	}
	if !charged.IsCharged(weaponObjID, registry.ShotSoulshot) {
		t.Error("weapon not charged after successful use")
	}
	// Weapon soulshot count = 1, so one shot consumed: 100 → 99 via Update.
	if repoFake.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", repoFake.updateCalls)
	}
	if repoFake.updated.Count != 99 {
		t.Errorf("remaining count = %d, want 99", repoFake.updated.Count)
	}
	if notifier.visualCall != 1 || notifier.visualID != 2039 || notifier.visualLvl != 1 {
		t.Errorf("visual = (calls=%d id=%d lvl=%d), want (1, 2039, 1)", notifier.visualCall, notifier.visualID, notifier.visualLvl)
	}
	if !notifier.sawSysMsg(342) { // ENABLED_SOULSHOT
		t.Errorf("expected ENABLED_SOULSHOT (342), got %v", notifier.sysMsgs)
	}
}

func TestShotHandler_AlreadyCharged_NoDoubleConsume(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	charged.SetCharged(weaponObjID, registry.ShotSoulshot, true) // pre-charged
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{weapon: &models.CharacterItem{ObjectID: weaponObjID, ItemID: weaponItemID}}
	h := newShotTestHandler(charged, notifier, noGradeWeapon())

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 1835, Count: 100}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: noGradeShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false when already charged")
	}
	if repoFake.updateCalls != 0 || repoFake.deleteCalls != 0 {
		t.Error("item consumed while already charged, want no re-consumption")
	}
}

func TestShotHandler_NotEnough_NoConsume(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	notifier := &recordingNotifier{}
	repoFake := &shotFakeItemRepo{weapon: &models.CharacterItem{ObjectID: weaponObjID, ItemID: weaponItemID}}
	// Weapon needs 2 shots per charge, but only 1 in the stack.
	weapon := &registry.ItemTemplate{ID: weaponItemID, CrystalType: registry.GradeNone, Soulshots: 2}
	h := newShotTestHandler(charged, notifier, weapon)

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 1835, Count: 1}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: noGradeShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false when not enough shots")
	}
	if charged.IsCharged(weaponObjID, registry.ShotSoulshot) {
		t.Error("weapon charged despite not enough shots")
	}
	if !notifier.sawSysMsg(338) { // NOT_ENOUGH_SOULSHOTS
		t.Errorf("expected NOT_ENOUGH_SOULSHOTS (338), got %v", notifier.sysMsgs)
	}
}

func TestShotHandler_LastShotDeletesStack(t *testing.T) {
	charged := registry.NewChargedShotRegistry()
	repoFake := &shotFakeItemRepo{weapon: &models.CharacterItem{ObjectID: weaponObjID, ItemID: weaponItemID}}
	h := newShotTestHandler(charged, nil, noGradeWeapon())

	shot := &models.CharacterItem{ObjectID: shotObjID, ItemID: 1835, Count: 1} // exactly one left
	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID: 7, Item: shot, Template: noGradeShot(), Repo: &shotFakeDB{item: repoFake},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true")
	}
	if repoFake.deleteCalls != 1 || repoFake.deletedID != shotObjID {
		t.Errorf("expected the empty stack to be deleted (calls=%d id=%d)", repoFake.deleteCalls, repoFake.deletedID)
	}
}

func TestGradeSPlus_CollapsesTopGrades(t *testing.T) {
	cases := []struct {
		in   registry.ItemGrade
		want registry.ItemGrade
	}{
		{registry.GradeNone, registry.GradeNone},
		{registry.GradeD, registry.GradeD},
		{registry.GradeS, registry.GradeS},
		{registry.GradeS80, registry.GradeS},
		{registry.GradeS84, registry.GradeS},
	}
	for _, c := range cases {
		if got := gradeSPlus(c.in); got != c.want {
			t.Errorf("gradeSPlus(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
