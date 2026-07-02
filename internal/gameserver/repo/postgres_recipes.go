package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// RecipeRepositoryImpl implements RecipeRepository for PostgreSQL.
type RecipeRepositoryImpl struct {
	db pgxDB
}

// NewRecipeRepository creates a recipe repository with pool.
func NewRecipeRepository(db pgxDB) *RecipeRepositoryImpl {
	return &RecipeRepositoryImpl{db: db}
}

// NewRecipeRepositoryTx creates a recipe repository with transaction.
func NewRecipeRepositoryTx(tx pgx.Tx) *RecipeRepositoryImpl {
	return &RecipeRepositoryImpl{db: tx}
}

// GetByCharacter returns all recipes registered by a character.
func (r *RecipeRepositoryImpl) GetByCharacter(ctx context.Context, charID int32) ([]models.CharacterRecipe, error) {
	rows, err := r.db.Query(ctx,
		`SELECT char_id, recipe_id, is_dwarven, class_index, registered_at
		 FROM character_recipes
		 WHERE char_id = $1
		 ORDER BY recipe_id`, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to query character recipes: %w", err)
	}
	defer rows.Close()

	var recipes []models.CharacterRecipe
	for rows.Next() {
		var rec models.CharacterRecipe
		if err := rows.Scan(&rec.CharID, &rec.RecipeID, &rec.IsDwarven, &rec.ClassIndex, &rec.RegisteredAt); err != nil {
			return nil, fmt.Errorf("failed to scan character recipe: %w", err)
		}
		recipes = append(recipes, rec)
	}
	return recipes, rows.Err()
}

// HasRecipe reports whether the character already registered the recipe.
func (r *RecipeRepositoryImpl) HasRecipe(ctx context.Context, charID int32, recipeID int32) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM character_recipes WHERE char_id = $1 AND recipe_id = $2)`,
		charID, recipeID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check recipe existence: %w", err)
	}
	return exists, nil
}

// CountByType returns how many recipes of a book (dwarven vs common) exist.
func (r *RecipeRepositoryImpl) CountByType(ctx context.Context, charID int32, isDwarven bool) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM character_recipes WHERE char_id = $1 AND is_dwarven = $2`,
		charID, isDwarven).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count recipes: %w", err)
	}
	return count, nil
}

// AddRecipe registers a recipe in the character's book. Idempotent on the primary
// key (char_id, recipe_id, class_index) so a race re-insert is a no-op.
func (r *RecipeRepositoryImpl) AddRecipe(ctx context.Context, charID int32, recipeID int32, isDwarven bool) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_recipes (char_id, recipe_id, is_dwarven, class_index)
		 VALUES ($1, $2, $3, 0)
		 ON CONFLICT (char_id, recipe_id, class_index) DO NOTHING`,
		charID, recipeID, isDwarven)
	if err != nil {
		return fmt.Errorf("failed to add recipe: %w", err)
	}
	return nil
}

// RemoveRecipe unregisters a recipe from the character's book.
func (r *RecipeRepositoryImpl) RemoveRecipe(ctx context.Context, charID int32, recipeID int32) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM character_recipes WHERE char_id = $1 AND recipe_id = $2`,
		charID, recipeID)
	if err != nil {
		return fmt.Errorf("failed to remove recipe: %w", err)
	}
	return nil
}

// DeleteByCharacter removes all recipes for a character.
func (r *RecipeRepositoryImpl) DeleteByCharacter(ctx context.Context, charID int32) error {
	_, err := r.db.Exec(ctx, `DELETE FROM character_recipes WHERE char_id = $1`, charID)
	if err != nil {
		return fmt.Errorf("failed to delete character recipes: %w", err)
	}
	return nil
}
