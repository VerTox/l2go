package usecase

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// --- fakes ---------------------------------------------------------------

type enchantFakeItemRepo struct {
	repo.ItemRepository
	items   map[int32]*models.CharacterItem
	updated map[int32]*models.CharacterItem
	deleted map[int32]bool
}

func newEnchantFakeItemRepo() *enchantFakeItemRepo {
	return &enchantFakeItemRepo{
		items:   map[int32]*models.CharacterItem{},
		updated: map[int32]*models.CharacterItem{},
		deleted: map[int32]bool{},
	}
}

func (f *enchantFakeItemRepo) GetByObjectID(_ context.Context, objectID int32) (*models.CharacterItem, error) {
	return f.items[objectID], nil
}
func (f *enchantFakeItemRepo) Update(_ context.Context, item *models.CharacterItem) error {
	f.updated[item.ObjectID] = item
	return nil
}
func (f *enchantFakeItemRepo) Delete(_ context.Context, objectID int32) error {
	f.deleted[objectID] = true
	return nil
}

type enchantFakeDB struct {
	repo.DatabaseRepository
	item repo.ItemRepository
}

func (f *enchantFakeDB) Item() repo.ItemRepository { return f.item }

type fakeEnchantData struct{ m map[int32]registry.EnchantScrollData }

func (f fakeEnchantData) GetEnchantScroll(id int32) (registry.EnchantScrollData, bool) {
	d, ok := f.m[id]
	return d, ok
}

type recEnchantNotifier struct {
	chooseItemIDs []int32
	sysMsgs       []int32
}

func (n *recEnchantNotifier) ChooseInventoryItem(_ int32, itemID int32) {
	n.chooseItemIDs = append(n.chooseItemIDs, itemID)
}
func (n *recEnchantNotifier) SystemMessage(_ int32, msgID int32) {
	n.sysMsgs = append(n.sysMsgs, msgID)
}

// fakeChanceSource returns a fixed chance (ok=true), or ok=false when notFound.
type fakeChanceSource struct {
	chance   float64
	notFound bool
}

func (f fakeChanceSource) Chance(_ int, _ int32, _ bool, _ int32, _ int) (float64, bool) {
	if f.notFound {
		return 0, false
	}
	return f.chance, true
}

// --- pure decision -------------------------------------------------------

func TestDecideEnchant(t *testing.T) {
	// baseChance 70 mirrors a fighter weapon at enchant level 3-14.
	weaponTarget := enchantTarget{type2: registry.ItemType2Weapon, grade: registry.GradeD, enchantLevel: 3, enchantable: true, baseChance: 70}
	normalScroll := enchantScrollInfo{isWeapon: true, targetGrade: registry.GradeD, maxEnchant: 65535}

	tests := []struct {
		name    string
		scroll  enchantScrollInfo
		target  enchantTarget
		roll    float64
		want    EnchantResultCode
		wantLvl int
	}{
		{"success increments level", normalScroll, weaponTarget, 10, EnchantCodeSuccess, 4}, // fighter lvl3 chance 70, roll<70
		{"normal fail destroys", normalScroll, weaponTarget, 80, EnchantCodeFailDestroy, 0},
		{
			"safe fail keeps level",
			enchantScrollInfo{isWeapon: true, isSafe: true, targetGrade: registry.GradeD, maxEnchant: 65535},
			weaponTarget, 80, EnchantCodeSafeFail, 3,
		},
		{
			"blessed fail resets level",
			enchantScrollInfo{isWeapon: true, isBlessed: true, targetGrade: registry.GradeD, maxEnchant: 65535},
			weaponTarget, 80, EnchantCodeBlessedFail, 0,
		},
		{
			"wrong type errors",
			normalScroll,
			enchantTarget{type2: registry.ItemType2Armor, grade: registry.GradeD, enchantable: true},
			0, EnchantCodeError, 0,
		},
		{
			"grade mismatch errors",
			normalScroll,
			enchantTarget{type2: registry.ItemType2Weapon, grade: registry.GradeC, enchantable: true},
			0, EnchantCodeError, 0,
		},
		{
			"not enchantable errors",
			normalScroll,
			enchantTarget{type2: registry.ItemType2Weapon, grade: registry.GradeD, enchantable: false},
			0, EnchantCodeError, 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decideEnchant(tc.scroll, tc.target, tc.roll)
			if got.code != tc.want {
				t.Fatalf("code = %d, want %d", got.code, tc.want)
			}
			if got.code == EnchantCodeSuccess && got.newLevel != tc.wantLvl {
				t.Fatalf("newLevel = %d, want %d", got.newLevel, tc.wantLvl)
			}
			// Errors must never consume the scroll.
			if got.code == EnchantCodeError && got.consumeScroll {
				t.Fatalf("error outcome must not consume scroll")
			}
			if got.code != EnchantCodeError && !got.consumeScroll {
				t.Fatalf("valid outcome must consume scroll")
			}
		})
	}
}

// --- use case flow -------------------------------------------------------

const (
	enchScrollObjID = int32(9001)
	enchTargetObjID = int32(9002)
	enchCharID      = int32(42)
	enchScrollItem  = int32(955)
	enchTargetItem  = int32(100)
)

func newEnchantUseCaseForTest(t *testing.T, scrollEtc string, roll float64) (*EnchantUseCase, *enchantFakeItemRepo, *registry.EnchantStateRegistry, *recEnchantNotifier) {
	t.Helper()
	itemRepo := newEnchantFakeItemRepo()
	itemRepo.items[enchScrollObjID] = &models.CharacterItem{ObjectID: enchScrollObjID, OwnerID: enchCharID, ItemID: enchScrollItem, Count: 5}
	itemRepo.items[enchTargetObjID] = &models.CharacterItem{ObjectID: enchTargetObjID, OwnerID: enchCharID, ItemID: enchTargetItem, Count: 1, EnchantLevel: 3}

	db := &enchantFakeDB{item: itemRepo}
	data := fakeEnchantData{m: map[int32]registry.EnchantScrollData{
		enchScrollItem: {TargetGrade: registry.GradeD, MaxEnchant: 65535},
	}}
	state := registry.NewEnchantStateRegistry()
	notifier := &recEnchantNotifier{}

	// Fixed base chance 70 so roll=10 succeeds and roll=99 fails, matching a
	// fighter weapon at enchant level 3.
	chances := fakeChanceSource{chance: 70}
	uc := NewEnchantUseCase(db, data, chances, state, notifier, func() float64 { return roll })
	uc.itemTemplate = func(itemID int32) *registry.ItemTemplate {
		switch itemID {
		case enchScrollItem:
			return &registry.ItemTemplate{ID: enchScrollItem, Type: "EtcItem", EtcItemType: registry.EtcItemType(scrollEtc)}
		case enchTargetItem:
			return &registry.ItemTemplate{ID: enchTargetItem, Type: "Weapon", Type2: registry.ItemType2Weapon, CrystalType: registry.GradeD, Enchantable: true}
		}
		return nil
	}
	return uc, itemRepo, state, notifier
}

func TestScrollHandler_ArmsAndPrompts(t *testing.T) {
	uc, _, state, notifier := newEnchantUseCaseForTest(t, "SCRL_ENCHANT_WP", 0)
	h := uc.ScrollHandler()

	consumed, err := h.UseItem(context.Background(), ItemUseContext{
		CharID:   enchCharID,
		Item:     &models.CharacterItem{ObjectID: enchScrollObjID, ItemID: enchScrollItem, Count: 5},
		Template: &registry.ItemTemplate{ID: enchScrollItem, EtcItemType: "SCRL_ENCHANT_WP"},
	})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed {
		t.Fatalf("scroll must NOT be consumed on arming (consumed=false)")
	}
	if !state.HasActive(enchCharID) {
		t.Fatalf("scroll should be armed on player state")
	}
	if len(notifier.chooseItemIDs) != 1 || notifier.chooseItemIDs[0] != enchScrollItem {
		t.Fatalf("expected ChooseInventoryItem for scroll %d, got %v", enchScrollItem, notifier.chooseItemIDs)
	}
}

func TestScrollHandler_AlreadyEnchanting(t *testing.T) {
	uc, _, state, notifier := newEnchantUseCaseForTest(t, "SCRL_ENCHANT_WP", 0)
	state.SetActive(enchCharID, registry.ActiveEnchant{ScrollObjectID: 1, ScrollItemID: enchScrollItem})
	h := uc.ScrollHandler()

	consumed, _ := h.UseItem(context.Background(), ItemUseContext{
		CharID:   enchCharID,
		Item:     &models.CharacterItem{ObjectID: enchScrollObjID, ItemID: enchScrollItem, Count: 5},
		Template: &registry.ItemTemplate{ID: enchScrollItem, EtcItemType: "SCRL_ENCHANT_WP"},
	})
	if consumed {
		t.Fatalf("consumed must be false")
	}
	if len(notifier.chooseItemIDs) != 0 {
		t.Fatalf("must not prompt again while enchanting")
	}
	if len(notifier.sysMsgs) != 1 || notifier.sysMsgs[0] != sysMsgEnchantInProgress {
		t.Fatalf("expected in-progress system message, got %v", notifier.sysMsgs)
	}
}

func TestEnchantItem_Success(t *testing.T) {
	uc, itemRepo, state, _ := newEnchantUseCaseForTest(t, "SCRL_ENCHANT_WP", 0.1) // roll=10 < 70
	state.SetActive(enchCharID, registry.ActiveEnchant{ScrollObjectID: enchScrollObjID, ScrollItemID: enchScrollItem})

	out, err := uc.EnchantItem(context.Background(), enchCharID, enchTargetObjID)
	if err != nil {
		t.Fatalf("EnchantItem error: %v", err)
	}
	if out.Code != EnchantCodeSuccess {
		t.Fatalf("code = %d, want success", out.Code)
	}
	if out.NewEnchantLevel != 4 {
		t.Fatalf("new level = %d, want 4", out.NewEnchantLevel)
	}
	if got := itemRepo.updated[enchTargetObjID]; got == nil || got.EnchantLevel != 4 {
		t.Fatalf("target enchant level not persisted as 4: %+v", got)
	}
	// Scroll consumed by one (5 -> 4).
	if got := itemRepo.updated[enchScrollObjID]; got == nil || got.Count != 4 {
		t.Fatalf("scroll count not decremented to 4: %+v", got)
	}
	if state.HasActive(enchCharID) {
		t.Fatalf("active enchant state must be cleared after attempt")
	}
}

func TestEnchantItem_NormalFailDestroys(t *testing.T) {
	uc, itemRepo, _, _ := newEnchantUseCaseForTest(t, "SCRL_ENCHANT_WP", 0.99) // roll=99 >= 70 -> fail
	uc.state.SetActive(enchCharID, registry.ActiveEnchant{ScrollObjectID: enchScrollObjID, ScrollItemID: enchScrollItem})

	out, err := uc.EnchantItem(context.Background(), enchCharID, enchTargetObjID)
	if err != nil {
		t.Fatalf("EnchantItem error: %v", err)
	}
	if out.Code != EnchantCodeFailDestroy {
		t.Fatalf("code = %d, want fail-destroy", out.Code)
	}
	if !itemRepo.deleted[enchTargetObjID] {
		t.Fatalf("target must be destroyed on normal-scroll failure")
	}
	if !out.TargetDestroyed {
		t.Fatalf("outcome.TargetDestroyed should be true")
	}
	// Scroll still consumed.
	if got := itemRepo.updated[enchScrollObjID]; got == nil || got.Count != 4 {
		t.Fatalf("scroll must be consumed on failure: %+v", got)
	}
}

func TestEnchantItem_BlessedFailResets(t *testing.T) {
	uc, itemRepo, _, _ := newEnchantUseCaseForTest(t, "BLESS_SCRL_ENCHANT_WP", 0.99)
	uc.state.SetActive(enchCharID, registry.ActiveEnchant{ScrollObjectID: enchScrollObjID, ScrollItemID: enchScrollItem})

	out, err := uc.EnchantItem(context.Background(), enchCharID, enchTargetObjID)
	if err != nil {
		t.Fatalf("EnchantItem error: %v", err)
	}
	if out.Code != EnchantCodeBlessedFail {
		t.Fatalf("code = %d, want blessed-fail", out.Code)
	}
	if itemRepo.deleted[enchTargetObjID] {
		t.Fatalf("blessed failure must NOT destroy item")
	}
	if got := itemRepo.updated[enchTargetObjID]; got == nil || got.EnchantLevel != 0 {
		t.Fatalf("blessed failure must reset enchant level to 0: %+v", got)
	}
}

func TestEnchantItem_SafeFailUnchanged(t *testing.T) {
	uc, itemRepo, _, _ := newEnchantUseCaseForTest(t, "ANCIENT_CRYSTAL_ENCHANT_WP", 0.99)
	uc.state.SetActive(enchCharID, registry.ActiveEnchant{ScrollObjectID: enchScrollObjID, ScrollItemID: enchScrollItem})

	out, err := uc.EnchantItem(context.Background(), enchCharID, enchTargetObjID)
	if err != nil {
		t.Fatalf("EnchantItem error: %v", err)
	}
	if out.Code != EnchantCodeSafeFail {
		t.Fatalf("code = %d, want safe-fail", out.Code)
	}
	if itemRepo.deleted[enchTargetObjID] {
		t.Fatalf("safe failure must NOT destroy item")
	}
	if got := itemRepo.updated[enchTargetObjID]; got == nil || got.EnchantLevel != 3 {
		t.Fatalf("safe failure must keep enchant level 3: %+v", got)
	}
}

func TestEnchantItem_WrongTargetType_NoConsume(t *testing.T) {
	uc, itemRepo, _, _ := newEnchantUseCaseForTest(t, "SCRL_ENCHANT_WP", 0)
	// Make the target an armor so the weapon scroll is invalid.
	uc.itemTemplate = func(itemID int32) *registry.ItemTemplate {
		switch itemID {
		case enchScrollItem:
			return &registry.ItemTemplate{ID: enchScrollItem, EtcItemType: "SCRL_ENCHANT_WP"}
		case enchTargetItem:
			return &registry.ItemTemplate{ID: enchTargetItem, Type: "Armor", Type2: registry.ItemType2Armor, CrystalType: registry.GradeD, Enchantable: true}
		}
		return nil
	}
	uc.state.SetActive(enchCharID, registry.ActiveEnchant{ScrollObjectID: enchScrollObjID, ScrollItemID: enchScrollItem})

	out, err := uc.EnchantItem(context.Background(), enchCharID, enchTargetObjID)
	if err != nil {
		t.Fatalf("EnchantItem error: %v", err)
	}
	if out.Code != EnchantCodeError {
		t.Fatalf("code = %d, want error", out.Code)
	}
	if itemRepo.deleted[enchTargetObjID] || len(itemRepo.deleted) != 0 {
		t.Fatalf("invalid enchant must not delete anything")
	}
	if len(itemRepo.updated) != 0 {
		t.Fatalf("invalid enchant must not consume scroll (no updates)")
	}
}

func TestEnchantItem_NoActiveScroll(t *testing.T) {
	uc, _, _, _ := newEnchantUseCaseForTest(t, "SCRL_ENCHANT_WP", 0)
	out, err := uc.EnchantItem(context.Background(), enchCharID, enchTargetObjID)
	if err != nil {
		t.Fatalf("EnchantItem error: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil outcome when no scroll armed, got %+v", out)
	}
}
