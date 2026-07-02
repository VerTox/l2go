package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func writeRecipes(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recipes.xml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

const sampleRecipes = `<list>
	<item id="1" recipeId="1666" name="mk_wooden_arrow" craftLevel="1" type="dwarven" successRate="100">
		<ingredient id="1864" count="4" />
		<production id="17" count="500" />
	</item>
	<item id="680" recipeId="6920" name="mk_fish_oil_average" craftLevel="2" type="common" successRate="100">
		<production id="6917" count="1" />
	</item>
</list>`

func TestRecipeRegistry_LookupByScrollItemID(t *testing.T) {
	root := t.TempDir()
	writeRecipes(t, root, sampleRecipes)

	reg := NewRecipeRegistry([]string{root})

	// Dwarven recipe resolved by its scroll item id (recipeId attr = 1666).
	rp, ok := reg.GetByItemID(1666)
	if !ok {
		t.Fatal("GetByItemID(1666) not found")
	}
	if rp.ID != 1 {
		t.Errorf("ID = %d, want 1 (internal recipe id)", rp.ID)
	}
	if rp.ItemID != 1666 {
		t.Errorf("ItemID = %d, want 1666 (scroll item id)", rp.ItemID)
	}
	if !rp.IsDwarven {
		t.Error("IsDwarven = false, want true for type=dwarven")
	}
	if rp.CraftLevel != 1 {
		t.Errorf("CraftLevel = %d, want 1", rp.CraftLevel)
	}
}

func TestRecipeRegistry_CommonRecipe(t *testing.T) {
	root := t.TempDir()
	writeRecipes(t, root, sampleRecipes)

	reg := NewRecipeRegistry([]string{root})
	rp, ok := reg.GetByItemID(6920)
	if !ok {
		t.Fatal("GetByItemID(6920) not found")
	}
	if rp.IsDwarven {
		t.Error("IsDwarven = true, want false for type=common")
	}
	if rp.CraftLevel != 2 {
		t.Errorf("CraftLevel = %d, want 2", rp.CraftLevel)
	}
}

func TestRecipeRegistry_UnknownItemID(t *testing.T) {
	root := t.TempDir()
	writeRecipes(t, root, sampleRecipes)

	reg := NewRecipeRegistry([]string{root})
	if _, ok := reg.GetByItemID(9999); ok {
		t.Error("GetByItemID(9999) returned ok=true for unknown scroll")
	}
	if reg.Count() != 2 {
		t.Errorf("Count = %d, want 2", reg.Count())
	}
}

func TestRecipeRegistry_MissingFileIsEmpty(t *testing.T) {
	reg := NewRecipeRegistry([]string{t.TempDir()})
	if _, ok := reg.GetByItemID(1666); ok {
		t.Error("expected no recipes when recipes.xml is absent")
	}
	if reg.Count() != 0 {
		t.Errorf("Count = %d, want 0", reg.Count())
	}
}

// Falls back to the second root when the first has no recipes.xml.
func TestRecipeRegistry_RootFallback(t *testing.T) {
	empty := t.TempDir()
	real := t.TempDir()
	writeRecipes(t, real, sampleRecipes)

	reg := NewRecipeRegistry([]string{empty, real})
	if _, ok := reg.GetByItemID(1666); !ok {
		t.Error("expected recipe from second root")
	}
}
