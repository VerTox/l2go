package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// SkillRepositoryImpl implements SkillRepository for PostgreSQL
type SkillRepositoryImpl struct {
	db pgxDB
}

// NewSkillRepository creates a skill repository with pool
func NewSkillRepository(db pgxDB) *SkillRepositoryImpl {
	return &SkillRepositoryImpl{db: db}
}

// NewSkillRepositoryTx creates a skill repository with transaction
func NewSkillRepositoryTx(tx pgx.Tx) *SkillRepositoryImpl {
	return &SkillRepositoryImpl{db: tx}
}

// GetByCharacter retrieves all skills for a character
func (r *SkillRepositoryImpl) GetByCharacter(ctx context.Context, charID int32) ([]models.CharacterSkill, error) {
	query := `
		SELECT char_id, skill_id, skill_level, class_index, learned_at
		FROM character_skills 
		WHERE char_id = $1 
		ORDER BY skill_id`

	rows, err := r.db.Query(ctx, query, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to query character skills: %w", err)
	}
	defer rows.Close()

	var skills []models.CharacterSkill
	for rows.Next() {
		var skill models.CharacterSkill

		err := rows.Scan(
			&skill.CharID, &skill.SkillID, &skill.SkillLevel,
			&skill.ClassIndex, &skill.LearnedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan skill: %w", err)
		}

		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

// GetSkill retrieves a specific skill for a character
func (r *SkillRepositoryImpl) GetSkill(ctx context.Context, charID int32, skillID int32) (*models.CharacterSkill, error) {
	query := `
		SELECT char_id, skill_id, skill_level, class_index, learned_at
		FROM character_skills 
		WHERE char_id = $1 AND skill_id = $2`

	var skill models.CharacterSkill

	err := r.db.QueryRow(ctx, query, charID, skillID).Scan(
		&skill.CharID, &skill.SkillID, &skill.SkillLevel,
		&skill.ClassIndex, &skill.LearnedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}

	return &skill, nil
}

// LearnSkill adds a new skill for a character
func (r *SkillRepositoryImpl) LearnSkill(ctx context.Context, charID int32, skillID int32, level int) error {
	query := `
		INSERT INTO character_skills (char_id, skill_id, skill_level, class_index, learned_at)
		VALUES ($1, $2, $3, 0, $4)
		ON CONFLICT (char_id, skill_id, class_index) 
		DO UPDATE SET skill_level = $3, learned_at = $4`

	_, err := r.db.Exec(ctx, query, charID, skillID, level, time.Now())
	if err != nil {
		return fmt.Errorf("failed to learn skill: %w", err)
	}

	return nil
}

// UpdateSkill updates an existing skill level
func (r *SkillRepositoryImpl) UpdateSkill(ctx context.Context, charID int32, skillID int32, level int) error {
	_, err := r.db.Exec(ctx,
		"UPDATE character_skills SET skill_level = $3, learned_at = $4 WHERE char_id = $1 AND skill_id = $2",
		charID, skillID, level, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update skill: %w", err)
	}
	return nil
}

// ForgetSkill removes a skill from a character
func (r *SkillRepositoryImpl) ForgetSkill(ctx context.Context, charID int32, skillID int32) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM character_skills WHERE char_id = $1 AND skill_id = $2",
		charID, skillID)
	if err != nil {
		return fmt.Errorf("failed to forget skill: %w", err)
	}
	return nil
}

// DeleteByCharacter deletes all skills for a character (used when character is deleted)
func (r *SkillRepositoryImpl) DeleteByCharacter(ctx context.Context, charID int32) error {
	_, err := r.db.Exec(ctx, "DELETE FROM character_skills WHERE char_id = $1", charID)
	if err != nil {
		return fmt.Errorf("failed to delete character skills: %w", err)
	}
	return nil
}

// HasSkill checks if character has a specific skill
func (r *SkillRepositoryImpl) HasSkill(ctx context.Context, charID int32, skillID int32) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM character_skills WHERE char_id = $1 AND skill_id = $2",
		charID, skillID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check skill existence: %w", err)
	}
	return count > 0, nil
}

// GetSkillLevel returns the level of a specific skill
func (r *SkillRepositoryImpl) GetSkillLevel(ctx context.Context, charID int32, skillID int32) (int, error) {
	var level int
	err := r.db.QueryRow(ctx,
		"SELECT skill_level FROM character_skills WHERE char_id = $1 AND skill_id = $2",
		charID, skillID).Scan(&level)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get skill level: %w", err)
	}
	return level, nil
}

// GetSkillsByType retrieves skills by type (would require skill template data)
func (r *SkillRepositoryImpl) GetSkillsByType(ctx context.Context, charID int32, skillType int) ([]models.CharacterSkill, error) {
	// For now, return all skills - would need skill template table to filter by type
	return r.GetByCharacter(ctx, charID)
}

// AddSkillEffect adds an active skill effect
func (r *SkillRepositoryImpl) AddSkillEffect(ctx context.Context, charID int32, skillID int32, remainingTime int) error {
	query := `
		INSERT INTO character_skill_effects (char_id, skill_id, skill_level, remaining_time, applied_at)
		VALUES ($1, $2, (SELECT skill_level FROM character_skills WHERE char_id = $1 AND skill_id = $2), $3, $4)
		ON CONFLICT (char_id, skill_id) 
		DO UPDATE SET remaining_time = $3, applied_at = $4`

	_, err := r.db.Exec(ctx, query, charID, skillID, remainingTime, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add skill effect: %w", err)
	}

	return nil
}

// RemoveSkillEffect removes an active skill effect
func (r *SkillRepositoryImpl) RemoveSkillEffect(ctx context.Context, charID int32, skillID int32) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM character_skill_effects WHERE char_id = $1 AND skill_id = $2",
		charID, skillID)
	if err != nil {
		return fmt.Errorf("failed to remove skill effect: %w", err)
	}
	return nil
}

// GetActiveEffects retrieves all active skill effects for a character
func (r *SkillRepositoryImpl) GetActiveEffects(ctx context.Context, charID int32) ([]models.CharacterSkillEffect, error) {
	query := `
		SELECT char_id, skill_id, skill_level, remaining_time, applied_at
		FROM character_skill_effects 
		WHERE char_id = $1 AND remaining_time > 0
		ORDER BY applied_at DESC`

	rows, err := r.db.Query(ctx, query, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to query skill effects: %w", err)
	}
	defer rows.Close()

	var effects []models.CharacterSkillEffect
	for rows.Next() {
		var effect models.CharacterSkillEffect

		err := rows.Scan(
			&effect.CharID, &effect.SkillID, &effect.SkillLevel,
			&effect.RemainingTime, &effect.AppliedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan skill effect: %w", err)
		}

		effects = append(effects, effect)
	}

	return effects, rows.Err()
}

// CleanupExpiredEffects removes expired skill effects
func (r *SkillRepositoryImpl) CleanupExpiredEffects(ctx context.Context) error {
	_, err := r.db.Exec(ctx, "DELETE FROM character_skill_effects WHERE remaining_time <= 0")
	if err != nil {
		return fmt.Errorf("failed to cleanup expired effects: %w", err)
	}
	return nil
}
