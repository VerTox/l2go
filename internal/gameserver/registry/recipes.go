package registry

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sync"
)

// Recipe is the minimal slice of an L2J L2RecipeList needed to register a recipe
// from a recipe scroll. It deliberately ignores ingredients / production / stat
// use (those belong to the crafting engine, which does not exist yet) and keeps
// only what the "Recipes" item handler needs.
//
// Naming trap (mirrors L2J): a recipe carries three different ids —
//   - ID:     the internal recipe-list id (recipes.xml <item id="..">). This is the
//     registration key stored in character_recipes and sent to the client.
//   - ItemID: the recipe SCROLL's item id (recipes.xml recipeId="..") — the item the
//     player double-clicks. This is the lookup key (L2J getRecipeByItemId).
//   - the crafted output item (recipes.xml <production id="..">) is intentionally
//     not modelled here.
type Recipe struct {
	ID         int32  // internal recipe-list id (registration key)
	ItemID     int32  // scroll item id (lookup key)
	Name       string // internal recipe name
	CraftLevel int    // required Create Dwarven/Common Item skill level
	IsDwarven  bool   // true = dwarven creation recipe, false = common recipe
}

// RecipeRegistry loads recipes.xml and indexes recipes by their scroll item id so
// the Recipes item handler can resolve a used scroll to the recipe it registers.
// Loading is lazy and happens once, on first lookup, from the first root that has
// a recipes.xml (roots are tried in order, matching the other data registries).
type RecipeRegistry struct {
	roots []string

	mu       sync.Mutex
	loaded   bool
	byItemID map[int32]*Recipe
}

// NewRecipeRegistry creates a registry that resolves recipes.xml under one of the
// given root directories (tried in order).
func NewRecipeRegistry(roots []string) *RecipeRegistry {
	return &RecipeRegistry{
		roots:    roots,
		byItemID: make(map[int32]*Recipe),
	}
}

// GetByItemID returns the recipe registered by the scroll with the given item id,
// loading recipes.xml on first access. Mirrors L2J RecipeData.getRecipeByItemId.
func (r *RecipeRegistry) GetByItemID(itemID int32) (*Recipe, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureLoaded()
	rp, ok := r.byItemID[itemID]
	return rp, ok
}

// Count returns the number of loaded recipes (loading on first access).
func (r *RecipeRegistry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureLoaded()
	return len(r.byItemID)
}

// ensureLoaded parses recipes.xml exactly once. Failure to find/parse the file is
// cached (empty index) so we never re-read a missing file on every lookup.
func (r *RecipeRegistry) ensureLoaded() {
	if r.loaded {
		return
	}
	r.loaded = true

	for _, root := range r.roots {
		data, err := os.ReadFile(filepath.Join(root, "recipes.xml"))
		if err != nil {
			continue
		}
		var list xmlRecipeList
		if err := xml.Unmarshal(data, &list); err != nil {
			continue
		}
		for i := range list.Items {
			it := &list.Items[i]
			rp := &Recipe{
				ID:         it.ID,
				ItemID:     it.RecipeID,
				Name:       it.Name,
				CraftLevel: it.CraftLevel,
				IsDwarven:  it.Type == "dwarven",
			}
			// Index by scroll item id; keep the first definition on duplicates.
			if _, exists := r.byItemID[rp.ItemID]; !exists {
				r.byItemID[rp.ItemID] = rp
			}
		}
		return
	}
}

// --- XML structures for L2J recipes.xml ---

type xmlRecipeList struct {
	XMLName xml.Name    `xml:"list"`
	Items   []xmlRecipe `xml:"item"`
}

type xmlRecipe struct {
	ID         int32  `xml:"id,attr"`
	RecipeID   int32  `xml:"recipeId,attr"`
	Name       string `xml:"name,attr"`
	CraftLevel int    `xml:"craftLevel,attr"`
	Type       string `xml:"type,attr"`
}
