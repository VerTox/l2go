package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// ShortcutRepositoryImpl implements ShortcutRepository for PostgreSQL
type ShortcutRepositoryImpl struct {
	db pgxDB
}

// NewShortcutRepository creates a shortcut repository with pool
func NewShortcutRepository(db pgxDB) *ShortcutRepositoryImpl {
	return &ShortcutRepositoryImpl{db: db}
}

// NewShortcutRepositoryTx creates a shortcut repository with transaction
func NewShortcutRepositoryTx(tx pgx.Tx) *ShortcutRepositoryImpl {
	return &ShortcutRepositoryImpl{db: tx}
}

// GetByCharacter retrieves all shortcuts for a character
func (r *ShortcutRepositoryImpl) GetByCharacter(ctx context.Context, charID int32) ([]models.CharacterShortcut, error) {
	query := `
		SELECT char_id, slot, page, type, shortcut_id, level, sub_level
		FROM character_shortcuts 
		WHERE char_id = $1 
		ORDER BY page, slot`

	rows, err := r.db.Query(ctx, query, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to query character shortcuts: %w", err)
	}
	defer rows.Close()

	var shortcuts []models.CharacterShortcut
	for rows.Next() {
		var shortcut models.CharacterShortcut

		err := rows.Scan(
			&shortcut.CharID, &shortcut.Slot, &shortcut.Page,
			&shortcut.Type, &shortcut.ShortcutID, &shortcut.Level,
			&shortcut.SubLevel,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan shortcut: %w", err)
		}

		shortcuts = append(shortcuts, shortcut)
	}

	return shortcuts, rows.Err()
}

// GetByPage retrieves shortcuts for a specific page
func (r *ShortcutRepositoryImpl) GetByPage(ctx context.Context, charID int32, page int) ([]models.CharacterShortcut, error) {
	query := `
		SELECT char_id, slot, page, type, shortcut_id, level, sub_level
		FROM character_shortcuts 
		WHERE char_id = $1 AND page = $2 
		ORDER BY slot`

	rows, err := r.db.Query(ctx, query, charID, page)
	if err != nil {
		return nil, fmt.Errorf("failed to query shortcuts by page: %w", err)
	}
	defer rows.Close()

	var shortcuts []models.CharacterShortcut
	for rows.Next() {
		var shortcut models.CharacterShortcut

		err := rows.Scan(
			&shortcut.CharID, &shortcut.Slot, &shortcut.Page,
			&shortcut.Type, &shortcut.ShortcutID, &shortcut.Level,
			&shortcut.SubLevel,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan shortcut by page: %w", err)
		}

		shortcuts = append(shortcuts, shortcut)
	}

	return shortcuts, rows.Err()
}

// GetBySlot retrieves shortcut at specific slot and page
func (r *ShortcutRepositoryImpl) GetBySlot(ctx context.Context, charID int32, slot, page int) (*models.CharacterShortcut, error) {
	query := `
		SELECT char_id, slot, page, type, shortcut_id, level, sub_level
		FROM character_shortcuts 
		WHERE char_id = $1 AND slot = $2 AND page = $3`

	var shortcut models.CharacterShortcut

	err := r.db.QueryRow(ctx, query, charID, slot, page).Scan(
		&shortcut.CharID, &shortcut.Slot, &shortcut.Page,
		&shortcut.Type, &shortcut.ShortcutID, &shortcut.Level,
		&shortcut.SubLevel,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get shortcut by slot: %w", err)
	}

	return &shortcut, nil
}

// SetShortcut creates or updates a shortcut
func (r *ShortcutRepositoryImpl) SetShortcut(ctx context.Context, shortcut *models.CharacterShortcut) error {
	query := `
		INSERT INTO character_shortcuts (char_id, slot, page, type, shortcut_id, level, sub_level)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (char_id, slot, page, class_index)
		DO UPDATE SET type = $4, shortcut_id = $5, level = $6, sub_level = $7`

	_, err := r.db.Exec(ctx, query,
		shortcut.CharID, shortcut.Slot, shortcut.Page,
		shortcut.Type, shortcut.ShortcutID, shortcut.Level,
		shortcut.SubLevel,
	)

	if err != nil {
		return fmt.Errorf("failed to set shortcut: %w", err)
	}

	return nil
}

// DeleteShortcut removes a shortcut at specific slot and page
func (r *ShortcutRepositoryImpl) DeleteShortcut(ctx context.Context, charID int32, slot, page int) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM character_shortcuts WHERE char_id = $1 AND slot = $2 AND page = $3",
		charID, slot, page)
	if err != nil {
		return fmt.Errorf("failed to delete shortcut: %w", err)
	}
	return nil
}

// DeleteByCharacter deletes all shortcuts for a character (used when character is deleted)
func (r *ShortcutRepositoryImpl) DeleteByCharacter(ctx context.Context, charID int32) error {
	_, err := r.db.Exec(ctx, "DELETE FROM character_shortcuts WHERE char_id = $1", charID)
	if err != nil {
		return fmt.Errorf("failed to delete character shortcuts: %w", err)
	}
	return nil
}

// ClearPage removes all shortcuts from a specific page
func (r *ShortcutRepositoryImpl) ClearPage(ctx context.Context, charID int32, page int) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM character_shortcuts WHERE char_id = $1 AND page = $2",
		charID, page)
	if err != nil {
		return fmt.Errorf("failed to clear shortcut page: %w", err)
	}
	return nil
}

// GetMaxPage returns the highest page number with shortcuts
func (r *ShortcutRepositoryImpl) GetMaxPage(ctx context.Context, charID int32) (int, error) {
	var maxPage int
	err := r.db.QueryRow(ctx,
		"SELECT COALESCE(MAX(page), 0) FROM character_shortcuts WHERE char_id = $1",
		charID).Scan(&maxPage)
	if err != nil {
		return 0, fmt.Errorf("failed to get max shortcut page: %w", err)
	}
	return maxPage, nil
}