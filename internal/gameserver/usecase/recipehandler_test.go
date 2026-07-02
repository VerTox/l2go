package usecase

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// --- test doubles ---

// fakeRecipeSource resolves recipes by scroll item id.
type fakeRecipeSource struct {
	byItem map[int32]*registry.Recipe
}

func (f *fakeRecipeSource) GetByItemID(itemID int32) (*registry.Recipe, bool) {
	rp, ok := f.byItem[itemID]
	return rp, ok
}

// recordingRecipeNotifier captures notifier calls.
type recordingRecipeNotifier struct {
	sysMsgs  []int32
	withInt  [][2]int32 // {msgID, value}
	itemMsgs [][2]int32 // {msgID, itemID}
}

func (n *recordingRecipeNotifier) SystemMessage(_ int32, msgID int32) {
	n.sysMsgs = append(n.sysMsgs, msgID)
}
func (n *recordingRecipeNotifier) SystemMessageWithInt(_ int32, msgID, value int32) {
	n.withInt = append(n.withInt, [2]int32{msgID, value})
}
func (n *recordingRecipeNotifier) ItemSystemMessage(_ int32, msgID, itemID int32) {
	n.itemMsgs = append(n.itemMsgs, [2]int32{msgID, itemID})
}

// fakeRecipeRepo implements repo.RecipeRepository for the handler tests.
type fakeRecipeRepo struct {
	repo.RecipeRepository // embedded: unused methods panic if called
	has                   map[int32]bool
	dwarvenCount          int
	commonCount           int
	added                 []int32 // recipe ids added
	addDwarven            []bool
}

func (f *fakeRecipeRepo) HasRecipe(_ context.Context, _ int32, recipeID int32) (bool, error) {
	return f.has[recipeID], nil
}
func (f *fakeRecipeRepo) CountByType(_ context.Context, _ int32, isDwarven bool) (int, error) {
	if isDwarven {
		return f.dwarvenCount, nil
	}
	return f.commonCount, nil
}
func (f *fakeRecipeRepo) AddRecipe(_ context.Context, _ int32, recipeID int32, isDwarven bool) error {
	f.added = append(f.added, recipeID)
	f.addDwarven = append(f.addDwarven, isDwarven)
	return nil
}

// fakeSkillRepo implements repo.SkillRepository for craft-ability checks.
type fakeSkillRepo struct {
	repo.SkillRepository // embedded
	levels               map[int32]int
}

func (f *fakeSkillRepo) GetSkillLevel(_ context.Context, _ int32, skillID int32) (int, error) {
	return f.levels[skillID], nil
}

// fakeRecipeItemRepo tracks item consumption.
type fakeRecipeItemRepo struct {
	repo.ItemRepository
	updated *models.CharacterItem
	deleted int32
}

func (f *fakeRecipeItemRepo) Update(_ context.Context, item *models.CharacterItem) error {
	f.updated = item
	return nil
}
func (f *fakeRecipeItemRepo) Delete(_ context.Context, objectID int32) error {
	f.deleted = objectID
	return nil
}

// fakeRecipeDB implements repo.DatabaseRepository exposing Item/Skill/Recipe.
type fakeRecipeDB struct {
	repo.DatabaseRepository
	item   *fakeRecipeItemRepo
	skill  *fakeSkillRepo
	recipe *fakeRecipeRepo
}

func (f *fakeRecipeDB) Item() repo.ItemRepository     { return f.item }
func (f *fakeRecipeDB) Skill() repo.SkillRepository   { return f.skill }
func (f *fakeRecipeDB) Recipe() repo.RecipeRepository { return f.recipe }

func newRecipeTest() (*RecipeHandler, *recordingRecipeNotifier, *fakeRecipeDB) {
	notifier := &recordingRecipeNotifier{}
	src := &fakeRecipeSource{byItem: map[int32]*registry.Recipe{
		// Dwarven recipe: scroll 1666 -> recipe id 1, craft level 1.
		1666: {ID: 1, ItemID: 1666, Name: "mk_wooden_arrow", CraftLevel: 1, IsDwarven: true},
		// Common recipe: scroll 6920 -> recipe id 680, craft level 2.
		6920: {ID: 680, ItemID: 6920, Name: "mk_fish_oil", CraftLevel: 2, IsDwarven: false},
	}}
	h := NewRecipeHandler(src, notifier)
	db := &fakeRecipeDB{
		item:   &fakeRecipeItemRepo{},
		skill:  &fakeSkillRepo{levels: map[int32]int{}},
		recipe: &fakeRecipeRepo{has: map[int32]bool{}},
	}
	return h, notifier, db
}

func TestRecipeHandler_RegistersDwarvenAndConsumes(t *testing.T) {
	h, notifier, db := newRecipeTest()
	db.skill.levels[createDwarvenSkillID] = 5 // has Create Dwarven Item lvl 5 >= craftLevel 1

	item := &models.CharacterItem{ObjectID: 900, ItemID: 1666, OwnerID: 7, Count: 2}
	tmpl := &registry.ItemTemplate{ID: 1666, Handler: "Recipes"}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: db})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true")
	}
	if len(db.recipe.added) != 1 || db.recipe.added[0] != 1 {
		t.Errorf("recipe added = %v, want [1]", db.recipe.added)
	}
	if len(db.recipe.addDwarven) != 1 || !db.recipe.addDwarven[0] {
		t.Errorf("addDwarven = %v, want [true]", db.recipe.addDwarven)
	}
	if item.Count != 1 {
		t.Errorf("item.Count = %d, want 1", item.Count)
	}
	if db.item.updated == nil || db.item.updated.Count != 1 {
		t.Errorf("expected item Update to count 1, got %+v", db.item.updated)
	}
	// S1_ADDED confirmation with item name.
	if len(notifier.itemMsgs) != 1 || notifier.itemMsgs[0] != [2]int32{sysMsgS1Added, 1666} {
		t.Errorf("itemMsgs = %v, want one S1_ADDED for item 1666", notifier.itemMsgs)
	}
}

func TestRecipeHandler_LastScrollDeleted(t *testing.T) {
	h, _, db := newRecipeTest()
	db.skill.levels[createCommonSkillID] = 2

	item := &models.CharacterItem{ObjectID: 901, ItemID: 6920, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{ID: 6920, Handler: "Recipes"}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: db})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if !consumed {
		t.Fatal("consumed = false, want true")
	}
	if item.Count != 0 || db.item.deleted != 901 {
		t.Errorf("expected scroll 901 deleted at count 0, count=%d deleted=%d", item.Count, db.item.deleted)
	}
	if len(db.recipe.addDwarven) != 1 || db.recipe.addDwarven[0] {
		t.Errorf("addDwarven = %v, want [false] for common recipe", db.recipe.addDwarven)
	}
}

func TestRecipeHandler_AlreadyRegistered_NoConsume(t *testing.T) {
	h, notifier, db := newRecipeTest()
	db.skill.levels[createDwarvenSkillID] = 5
	db.recipe.has[1] = true // recipe id 1 already registered

	item := &models.CharacterItem{ObjectID: 902, ItemID: 1666, OwnerID: 7, Count: 3}
	tmpl := &registry.ItemTemplate{ID: 1666, Handler: "Recipes"}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: db})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false when already registered")
	}
	if len(db.recipe.added) != 0 {
		t.Errorf("recipe should not be added again, got %v", db.recipe.added)
	}
	if item.Count != 3 || db.item.updated != nil || db.item.deleted != 0 {
		t.Error("scroll must not be consumed when already registered")
	}
	if len(notifier.sysMsgs) != 1 || notifier.sysMsgs[0] != sysMsgRecipeAlreadyRegistered {
		t.Errorf("sysMsgs = %v, want [RECIPE_ALREADY_REGISTERED]", notifier.sysMsgs)
	}
}

func TestRecipeHandler_InvalidRecipe_NoOp(t *testing.T) {
	h, notifier, db := newRecipeTest()
	db.skill.levels[createDwarvenSkillID] = 5

	item := &models.CharacterItem{ObjectID: 903, ItemID: 55555, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{ID: 55555, Handler: "Recipes"}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: db})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed {
		t.Error("consumed = true, want false for unknown recipe scroll")
	}
	if item.Count != 1 || len(db.recipe.added) != 0 {
		t.Error("nothing should happen for an unknown recipe scroll")
	}
	if len(notifier.sysMsgs)+len(notifier.itemMsgs)+len(notifier.withInt) != 0 {
		t.Error("no messages expected for unknown recipe scroll")
	}
}

func TestRecipeHandler_NoCraftAbility_NoConsume(t *testing.T) {
	h, notifier, db := newRecipeTest()
	// No craft skill learned (level 0).

	item := &models.CharacterItem{ObjectID: 904, ItemID: 1666, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{ID: 1666, Handler: "Recipes"}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: db})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed || len(db.recipe.added) != 0 || item.Count != 1 {
		t.Error("must not register/consume without craft ability")
	}
	if len(notifier.sysMsgs) != 1 || notifier.sysMsgs[0] != sysMsgCantRegisterNoAbility {
		t.Errorf("sysMsgs = %v, want [CANT_REGISTER_NO_ABILITY_TO_CRAFT]", notifier.sysMsgs)
	}
}

func TestRecipeHandler_CraftLevelTooLow_NoConsume(t *testing.T) {
	h, notifier, db := newRecipeTest()
	db.skill.levels[createCommonSkillID] = 1 // recipe craftLevel 2 > skill 1

	item := &models.CharacterItem{ObjectID: 905, ItemID: 6920, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{ID: 6920, Handler: "Recipes"}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: db})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed || len(db.recipe.added) != 0 || item.Count != 1 {
		t.Error("must not register/consume when craft level too low")
	}
	if len(notifier.sysMsgs) != 1 || notifier.sysMsgs[0] != sysMsgCreateLvlTooLow {
		t.Errorf("sysMsgs = %v, want [CREATE_LVL_TOO_LOW_TO_REGISTER]", notifier.sysMsgs)
	}
}

func TestRecipeHandler_LimitReached_NoConsume(t *testing.T) {
	h, notifier, db := newRecipeTest()
	db.skill.levels[createDwarvenSkillID] = 5
	db.recipe.dwarvenCount = defaultRecipeLimit // book full

	item := &models.CharacterItem{ObjectID: 906, ItemID: 1666, OwnerID: 7, Count: 1}
	tmpl := &registry.ItemTemplate{ID: 1666, Handler: "Recipes"}

	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: tmpl, Repo: db})
	if err != nil {
		t.Fatalf("UseItem error: %v", err)
	}
	if consumed || len(db.recipe.added) != 0 || item.Count != 1 {
		t.Error("must not register/consume when recipe book full")
	}
	if len(notifier.withInt) != 1 || notifier.withInt[0] != [2]int32{sysMsgUpToS1Recipes, defaultRecipeLimit} {
		t.Errorf("withInt = %v, want [UP_TO_S1_RECIPES_CAN_REGISTER, %d]", notifier.withInt, defaultRecipeLimit)
	}
}

// A nil notifier must not panic (mirrors the shot handler contract).
func TestRecipeHandler_NilNotifierSilentSuccess(t *testing.T) {
	src := &fakeRecipeSource{byItem: map[int32]*registry.Recipe{
		1666: {ID: 1, ItemID: 1666, CraftLevel: 1, IsDwarven: true},
	}}
	h := NewRecipeHandler(src, nil)
	db := &fakeRecipeDB{
		item:   &fakeRecipeItemRepo{},
		skill:  &fakeSkillRepo{levels: map[int32]int{createDwarvenSkillID: 3}},
		recipe: &fakeRecipeRepo{has: map[int32]bool{}},
	}
	item := &models.CharacterItem{ObjectID: 907, ItemID: 1666, OwnerID: 7, Count: 1}
	consumed, err := h.UseItem(context.Background(), ItemUseContext{CharID: 7, Item: item, Template: &registry.ItemTemplate{ID: 1666}, Repo: db})
	if err != nil || !consumed {
		t.Fatalf("nil-notifier success expected, consumed=%v err=%v", consumed, err)
	}
	if len(db.recipe.added) != 1 {
		t.Error("recipe should still be registered with nil notifier")
	}
}
