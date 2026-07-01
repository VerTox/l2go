package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// PostgreSQLRepository implements all repository interfaces using PostgreSQL
type PostgreSQLRepository struct {
	db       *pgxpool.Pool
	char     *CharacterRepositoryImpl
	item     *ItemRepositoryImpl
	skill    *SkillRepositoryImpl
	shortcut *ShortcutRepositoryImpl
	spawn    *SpawnRepositoryImpl
}

// NewPostgreSQLRepository creates a new PostgreSQL repository
func NewPostgreSQLRepository(db *pgxpool.Pool) DatabaseRepository {
	return &PostgreSQLRepository{
		db:       db,
		char:     NewCharacterRepository(db),
		item:     NewItemRepository(db),
		skill:    NewSkillRepository(db),
		shortcut: NewShortcutRepository(db),
		spawn:    NewSpawnRepository(db),
	}
}

// Repository access methods
func (r *PostgreSQLRepository) Character() CharacterRepository { return r.char }
func (r *PostgreSQLRepository) Item() ItemRepository           { return r.item }
func (r *PostgreSQLRepository) Skill() SkillRepository         { return r.skill }
func (r *PostgreSQLRepository) Shortcut() ShortcutRepository   { return r.shortcut }
func (r *PostgreSQLRepository) Spawn() SpawnRepository         { return r.spawn }

// Transaction implementation
type PostgreSQLTransaction struct {
	tx       pgx.Tx
	char     *CharacterRepositoryImpl
	item     *ItemRepositoryImpl
	skill    *SkillRepositoryImpl
	shortcut *ShortcutRepositoryImpl
}

func (t *PostgreSQLTransaction) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *PostgreSQLTransaction) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
func (t *PostgreSQLTransaction) Character() CharacterRepository     { return t.char }
func (t *PostgreSQLTransaction) Item() ItemRepository               { return t.item }
func (t *PostgreSQLTransaction) Skill() SkillRepository             { return t.skill }
func (t *PostgreSQLTransaction) Shortcut() ShortcutRepository       { return t.shortcut }

// BeginTransaction starts a new database transaction
func (r *PostgreSQLRepository) BeginTransaction(ctx context.Context) (Transaction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &PostgreSQLTransaction{
		tx:       tx,
		char:     NewCharacterRepositoryTx(tx),
		item:     NewItemRepositoryTx(tx),
		skill:    NewSkillRepositoryTx(tx),
		shortcut: NewShortcutRepositoryTx(tx),
	}, nil
}

// WithTransaction executes a function within a transaction
func (r *PostgreSQLRepository) WithTransaction(ctx context.Context, fn func(tx Transaction) error) error {
	tx, err := r.BeginTransaction(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("transaction error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit(ctx)
}

// HealthCheck verifies database connectivity
func (r *PostgreSQLRepository) HealthCheck(ctx context.Context) error {
	return r.db.Ping(ctx)
}

// CharacterRepositoryImpl implements CharacterRepository for PostgreSQL
type CharacterRepositoryImpl struct {
	db pgxDB
}

// pgxDB interface allows using both pgxpool.Pool and pgx.Tx
type pgxDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// NewCharacterRepository creates a character repository with pool
func NewCharacterRepository(db *pgxpool.Pool) *CharacterRepositoryImpl {
	return &CharacterRepositoryImpl{db: db}
}

// NewCharacterRepositoryTx creates a character repository with transaction
func NewCharacterRepositoryTx(tx pgx.Tx) *CharacterRepositoryImpl {
	return &CharacterRepositoryImpl{db: tx}
}

// GetByAccount retrieves all characters for an account
func (r *CharacterRepositoryImpl) GetByAccount(ctx context.Context, accountName string) ([]models.Character, error) {
	query := `
		SELECT char_id, account_name, char_name, level, max_hp, cur_hp, max_mp, cur_mp, max_cp, cur_cp,
			   face, hair_style, hair_color, sex, exp, sp, karma, pk_kills, pvp_kills, clan_id,
			   race, class_id, base_class, delete_time, vitality_points, access_level,
			   x, y, z, heading, created_at, last_access, online_time, online_status,
			   char_slot, newbie, noble, hero, hero_end_date, death_penalty_level,
			   title, rec_have, rec_left, fame, fishing_points,
			   base_str, base_dex, base_con, base_int, base_wit, base_men
		FROM characters
		WHERE account_name = $1
		ORDER BY char_slot ASC`

	rows, err := r.db.Query(ctx, query, accountName)
	if err != nil {
		return nil, fmt.Errorf("failed to query characters: %w", err)
	}
	defer rows.Close()

	var characters []models.Character
	for rows.Next() {
		var char models.Character
		var heroEndDate sql.NullTime

		err := rows.Scan(
			&char.ID, &char.AccountName, &char.Name, &char.Level,
			&char.MaxHP, &char.CurrentHP, &char.MaxMP, &char.CurrentMP,
			&char.MaxCP, &char.CurrentCP, &char.Face, &char.HairStyle,
			&char.HairColor, &char.Sex, &char.Experience, &char.SP,
			&char.Karma, &char.PKKills, &char.PvPKills, &char.ClanID,
			&char.Race, &char.ClassID, &char.BaseClass, &char.DeleteTime,
			&char.VitalityPoints, &char.AccessLevel, &char.Position.X,
			&char.Position.Y, &char.Position.Z, &char.Heading,
			&char.CreatedAt, &char.LastAccess, &char.OnlineTime,
			&char.OnlineStatus, &char.CharSlot, &char.Newbie, &char.Noble,
			&char.Hero, &heroEndDate, &char.DeathPenaltyLevel,
			&char.Title, &char.RecHave, &char.RecLeft, &char.Fame,
			&char.FishingPoints,
			&char.BaseSTR, &char.BaseDEX, &char.BaseCON,
			&char.BaseINT, &char.BaseWIT, &char.BaseMEN,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan character: %w", err)
		}

		if heroEndDate.Valid {
			char.HeroEndDate = &heroEndDate.Time
		}

		characters = append(characters, char)
	}

	return characters, rows.Err()
}

func (r *CharacterRepositoryImpl) GetByAccountSlotID(ctx context.Context, accountName string, charSlot int32) (*models.Character, error) {
	query := `
		SELECT char_id, account_name, char_name, level, max_hp, cur_hp, max_mp, cur_mp, max_cp, cur_cp,
			   face, hair_style, hair_color, sex, exp, sp, karma, pk_kills, pvp_kills, clan_id,
			   race, class_id, base_class, delete_time, vitality_points, access_level,
			   x, y, z, heading, created_at, last_access, online_time, online_status,
			   char_slot, newbie, noble, hero, hero_end_date, death_penalty_level,
			   title, rec_have, rec_left, fame, fishing_points,
			   base_str, base_dex, base_con, base_int, base_wit, base_men
		FROM characters
		WHERE char_slot = $1 AND account_name = $2`

	var char models.Character
	var heroEndDate sql.NullTime

	err := r.db.QueryRow(ctx, query, charSlot, accountName).Scan(
		&char.ID, &char.AccountName, &char.Name, &char.Level,
		&char.MaxHP, &char.CurrentHP, &char.MaxMP, &char.CurrentMP,
		&char.MaxCP, &char.CurrentCP, &char.Face, &char.HairStyle,
		&char.HairColor, &char.Sex, &char.Experience, &char.SP,
		&char.Karma, &char.PKKills, &char.PvPKills, &char.ClanID,
		&char.Race, &char.ClassID, &char.BaseClass, &char.DeleteTime,
		&char.VitalityPoints, &char.AccessLevel, &char.Position.X,
		&char.Position.Y, &char.Position.Z, &char.Heading,
		&char.CreatedAt, &char.LastAccess, &char.OnlineTime,
		&char.OnlineStatus, &char.CharSlot, &char.Newbie, &char.Noble,
		&char.Hero, &heroEndDate, &char.DeathPenaltyLevel,
		&char.Title, &char.RecHave, &char.RecLeft, &char.Fame,
		&char.FishingPoints,
		&char.BaseSTR, &char.BaseDEX, &char.BaseCON,
		&char.BaseINT, &char.BaseWIT, &char.BaseMEN,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get character by account slotID: %w", err)
	}

	if heroEndDate.Valid {
		char.HeroEndDate = &heroEndDate.Time
	}

	return &char, nil
}

// GetByID retrieves a character by ID
func (r *CharacterRepositoryImpl) GetByID(ctx context.Context, charID int32) (*models.Character, error) {
	query := `
		SELECT char_id, account_name, char_name, level, max_hp, cur_hp, max_mp, cur_mp, max_cp, cur_cp,
			   face, hair_style, hair_color, sex, exp, sp, karma, pk_kills, pvp_kills, clan_id,
			   race, class_id, base_class, delete_time, vitality_points, access_level,
			   x, y, z, heading, created_at, last_access, online_time, online_status,
			   char_slot, newbie, noble, hero, hero_end_date, death_penalty_level,
			   title, rec_have, rec_left, fame, fishing_points,
			   base_str, base_dex, base_con, base_int, base_wit, base_men
		FROM characters
		WHERE char_id = $1`

	var char models.Character
	var heroEndDate sql.NullTime

	err := r.db.QueryRow(ctx, query, charID).Scan(
		&char.ID, &char.AccountName, &char.Name, &char.Level,
		&char.MaxHP, &char.CurrentHP, &char.MaxMP, &char.CurrentMP,
		&char.MaxCP, &char.CurrentCP, &char.Face, &char.HairStyle,
		&char.HairColor, &char.Sex, &char.Experience, &char.SP,
		&char.Karma, &char.PKKills, &char.PvPKills, &char.ClanID,
		&char.Race, &char.ClassID, &char.BaseClass, &char.DeleteTime,
		&char.VitalityPoints, &char.AccessLevel, &char.Position.X,
		&char.Position.Y, &char.Position.Z, &char.Heading,
		&char.CreatedAt, &char.LastAccess, &char.OnlineTime,
		&char.OnlineStatus, &char.CharSlot, &char.Newbie, &char.Noble,
		&char.Hero, &heroEndDate, &char.DeathPenaltyLevel,
		&char.Title, &char.RecHave, &char.RecLeft, &char.Fame,
		&char.FishingPoints,
		&char.BaseSTR, &char.BaseDEX, &char.BaseCON,
		&char.BaseINT, &char.BaseWIT, &char.BaseMEN,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get character by ID: %w", err)
	}

	if heroEndDate.Valid {
		char.HeroEndDate = &heroEndDate.Time
	}

	return &char, nil
}

// GetByName retrieves a character by name
func (r *CharacterRepositoryImpl) GetByName(ctx context.Context, name string) (*models.Character, error) {
	query := `
		SELECT char_id, account_name, char_name, level, max_hp, cur_hp, max_mp, cur_mp, max_cp, cur_cp,
			   face, hair_style, hair_color, sex, exp, sp, karma, pk_kills, pvp_kills, clan_id,
			   race, class_id, base_class, delete_time, vitality_points, access_level,
			   x, y, z, heading, created_at, last_access, online_time, online_status,
			   char_slot, newbie, noble, hero, hero_end_date, death_penalty_level,
			   title, rec_have, rec_left, fame, fishing_points,
			   base_str, base_dex, base_con, base_int, base_wit, base_men
		FROM characters
		WHERE char_name = $1`

	var char models.Character
	var heroEndDate sql.NullTime

	err := r.db.QueryRow(ctx, query, name).Scan(
		&char.ID, &char.AccountName, &char.Name, &char.Level,
		&char.MaxHP, &char.CurrentHP, &char.MaxMP, &char.CurrentMP,
		&char.MaxCP, &char.CurrentCP, &char.Face, &char.HairStyle,
		&char.HairColor, &char.Sex, &char.Experience, &char.SP,
		&char.Karma, &char.PKKills, &char.PvPKills, &char.ClanID,
		&char.Race, &char.ClassID, &char.BaseClass, &char.DeleteTime,
		&char.VitalityPoints, &char.AccessLevel, &char.Position.X,
		&char.Position.Y, &char.Position.Z, &char.Heading,
		&char.CreatedAt, &char.LastAccess, &char.OnlineTime,
		&char.OnlineStatus, &char.CharSlot, &char.Newbie, &char.Noble,
		&char.Hero, &heroEndDate, &char.DeathPenaltyLevel,
		&char.Title, &char.RecHave, &char.RecLeft, &char.Fame,
		&char.FishingPoints,
		&char.BaseSTR, &char.BaseDEX, &char.BaseCON,
		&char.BaseINT, &char.BaseWIT, &char.BaseMEN,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get character by name: %w", err)
	}

	if heroEndDate.Valid {
		char.HeroEndDate = &heroEndDate.Time
	}

	return &char, nil
}

// Create creates a new character
func (r *CharacterRepositoryImpl) Create(ctx context.Context, char *models.Character) error {
	query := `
		INSERT INTO characters (
			account_name, char_name, level, max_hp, cur_hp, max_mp, cur_mp, max_cp, cur_cp,
			face, hair_style, hair_color, sex, exp, sp, karma, pk_kills, pvp_kills, clan_id,
			race, class_id, base_class, delete_time, vitality_points, access_level,
			x, y, z, heading, last_access, online_time, online_status,
			char_slot, newbie, noble, hero, hero_end_date, death_penalty_level,
			title, rec_have, rec_left, fame, fishing_points,
			base_str, base_dex, base_con, base_int, base_wit, base_men
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36,
			$37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47, $48, $49
		) RETURNING char_id, created_at`

	err := r.db.QueryRow(ctx, query,
		char.AccountName, char.Name, char.Level, char.MaxHP, char.CurrentHP,
		char.MaxMP, char.CurrentMP, char.MaxCP, char.CurrentCP, char.Face,
		char.HairStyle, char.HairColor, char.Sex, char.Experience, char.SP,
		char.Karma, char.PKKills, char.PvPKills, char.ClanID, char.Race,
		char.ClassID, char.BaseClass, char.DeleteTime, char.VitalityPoints,
		char.AccessLevel, char.Position.X, char.Position.Y, char.Position.Z,
		char.Heading, char.LastAccess, char.OnlineTime, char.OnlineStatus,
		char.CharSlot, char.Newbie, char.Noble, char.Hero, char.HeroEndDate,
		char.DeathPenaltyLevel, char.Title, char.RecHave, char.RecLeft,
		char.Fame, char.FishingPoints,
		char.BaseSTR, char.BaseDEX, char.BaseCON,
		char.BaseINT, char.BaseWIT, char.BaseMEN,
	).Scan(&char.ID, &char.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create character: %w", err)
	}

	return nil
}

// Update updates an existing character
func (r *CharacterRepositoryImpl) Update(ctx context.Context, char *models.Character) error {
	query := `
		UPDATE characters SET
			level = $2, max_hp = $3, cur_hp = $4, max_mp = $5, cur_mp = $6, max_cp = $7, cur_cp = $8,
			face = $9, hair_style = $10, hair_color = $11, sex = $12, exp = $13, sp = $14,
			karma = $15, pk_kills = $16, pvp_kills = $17, clan_id = $18, race = $19, class_id = $20,
			base_class = $21, delete_time = $22, vitality_points = $23, access_level = $24,
			x = $25, y = $26, z = $27, heading = $28, last_access = $29, online_time = $30,
			online_status = $31, char_slot = $32, newbie = $33, noble = $34, hero = $35,
			hero_end_date = $36, death_penalty_level = $37, title = $38, rec_have = $39,
			rec_left = $40, fame = $41, fishing_points = $42,
			base_str = $43, base_dex = $44, base_con = $45,
			base_int = $46, base_wit = $47, base_men = $48
		WHERE char_id = $1`

	_, err := r.db.Exec(ctx, query,
		char.ID, char.Level, char.MaxHP, char.CurrentHP, char.MaxMP,
		char.CurrentMP, char.MaxCP, char.CurrentCP, char.Face, char.HairStyle,
		char.HairColor, char.Sex, char.Experience, char.SP, char.Karma,
		char.PKKills, char.PvPKills, char.ClanID, char.Race, char.ClassID,
		char.BaseClass, char.DeleteTime, char.VitalityPoints, char.AccessLevel,
		char.Position.X, char.Position.Y, char.Position.Z, char.Heading,
		char.LastAccess, char.OnlineTime, char.OnlineStatus, char.CharSlot,
		char.Newbie, char.Noble, char.Hero, char.HeroEndDate,
		char.DeathPenaltyLevel, char.Title, char.RecHave, char.RecLeft,
		char.Fame, char.FishingPoints,
		char.BaseSTR, char.BaseDEX, char.BaseCON,
		char.BaseINT, char.BaseWIT, char.BaseMEN,
	)

	if err != nil {
		return fmt.Errorf("failed to update character: %w", err)
	}

	return nil
}

// Delete deletes a character by ID
func (r *CharacterRepositoryImpl) Delete(ctx context.Context, charID int32) error {
	_, err := r.db.Exec(ctx, "DELETE FROM characters WHERE char_id = $1", charID)
	if err != nil {
		return fmt.Errorf("failed to delete character: %w", err)
	}
	return nil
}

// GetCount returns total and in-deletion character count for account.
// Account matching is case-insensitive: the LoginServer lowercases account names
// (RequestCharacters sends e.g. "vertox"), while characters may be stored with the
// original case ("VerTox"), so a case-sensitive match would return 0. (l2go-rx4)
func (r *CharacterRepositoryImpl) GetCount(ctx context.Context, accountName string) (int, int, error) {
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE delete_time > 0) as in_deletion
		FROM characters
		WHERE LOWER(account_name) = LOWER($1)`

	var total, inDeletion int
	err := r.db.QueryRow(ctx, query, accountName).Scan(&total, &inDeletion)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get character count: %w", err)
	}

	return total, inDeletion, nil
}

// IsNameTaken checks if character name is already taken
func (r *CharacterRepositoryImpl) IsNameTaken(ctx context.Context, name string) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM characters WHERE char_name = $1", name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check name availability: %w", err)
	}
	return count > 0, nil
}

// GetBySlot retrieves character by account and slot
func (r *CharacterRepositoryImpl) GetBySlot(ctx context.Context, accountName string, slot int) (*models.Character, error) {
	chars, err := r.GetByAccount(ctx, accountName)
	if err != nil {
		return nil, err
	}

	for _, char := range chars {
		if char.CharSlot == slot {
			return &char, nil
		}
	}

	return nil, nil
}

// GetMaxSlot returns highest character slot for account
func (r *CharacterRepositoryImpl) GetMaxSlot(ctx context.Context, accountName string) (int, error) {
	var maxSlot sql.NullInt32
	err := r.db.QueryRow(ctx,
		"SELECT MAX(char_slot) FROM characters WHERE account_name = $1",
		accountName).Scan(&maxSlot)
	if err != nil {
		return 0, fmt.Errorf("failed to get max slot: %w", err)
	}

	if !maxSlot.Valid {
		return -1, nil
	}

	return int(maxSlot.Int32), nil
}

// GetDeleteCandidates returns characters ready for deletion
func (r *CharacterRepositoryImpl) GetDeleteCandidates(ctx context.Context) ([]models.Character, error) {
	now := time.Now().Unix()
	query := `
		SELECT char_id, account_name, char_name, level, max_hp, cur_hp, max_mp, cur_mp, max_cp, cur_cp,
			   face, hair_style, hair_color, sex, exp, sp, karma, pk_kills, pvp_kills, clan_id,
			   race, class_id, base_class, delete_time, vitality_points, access_level,
			   x, y, z, heading, created_at, last_access, online_time, online_status,
			   char_slot, newbie, noble, hero, hero_end_date, death_penalty_level,
			   title, rec_have, rec_left, fame, fishing_points,
			   base_str, base_dex, base_con, base_int, base_wit, base_men
		FROM characters
		WHERE delete_time > 0 AND delete_time <= $1`

	rows, err := r.db.Query(ctx, query, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get delete candidates: %w", err)
	}
	defer rows.Close()

	var characters []models.Character
	for rows.Next() {
		var char models.Character
		var heroEndDate sql.NullTime

		err := rows.Scan(
			&char.ID, &char.AccountName, &char.Name, &char.Level,
			&char.MaxHP, &char.CurrentHP, &char.MaxMP, &char.CurrentMP,
			&char.MaxCP, &char.CurrentCP, &char.Face, &char.HairStyle,
			&char.HairColor, &char.Sex, &char.Experience, &char.SP,
			&char.Karma, &char.PKKills, &char.PvPKills, &char.ClanID,
			&char.Race, &char.ClassID, &char.BaseClass, &char.DeleteTime,
			&char.VitalityPoints, &char.AccessLevel, &char.Position.X,
			&char.Position.Y, &char.Position.Z, &char.Heading,
			&char.CreatedAt, &char.LastAccess, &char.OnlineTime,
			&char.OnlineStatus, &char.CharSlot, &char.Newbie, &char.Noble,
			&char.Hero, &heroEndDate, &char.DeathPenaltyLevel,
			&char.Title, &char.RecHave, &char.RecLeft, &char.Fame,
			&char.FishingPoints,
			&char.BaseSTR, &char.BaseDEX, &char.BaseCON,
			&char.BaseINT, &char.BaseWIT, &char.BaseMEN,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan delete candidate: %w", err)
		}

		if heroEndDate.Valid {
			char.HeroEndDate = &heroEndDate.Time
		}

		characters = append(characters, char)
	}

	return characters, rows.Err()
}

// SetOnlineStatus updates character online status
func (r *CharacterRepositoryImpl) SetOnlineStatus(ctx context.Context, charID int32, online bool) error {
	_, err := r.db.Exec(ctx,
		"UPDATE characters SET online_status = $2 WHERE char_id = $1",
		charID, online)
	if err != nil {
		return fmt.Errorf("failed to set online status: %w", err)
	}
	return nil
}

// UpdatePosition updates character position and heading
func (r *CharacterRepositoryImpl) UpdatePosition(ctx context.Context, charID int32, x, y, z int, heading int) error {
	_, err := r.db.Exec(ctx,
		"UPDATE characters SET x = $2, y = $3, z = $4, heading = $5 WHERE char_id = $1",
		charID, x, y, z, heading)
	if err != nil {
		return fmt.Errorf("failed to update position: %w", err)
	}
	return nil
}

// UpdateLastAccess updates character last access time
func (r *CharacterRepositoryImpl) UpdateLastAccess(ctx context.Context, charID int32) error {
	now := time.Now().Unix()
	_, err := r.db.Exec(ctx,
		"UPDATE characters SET last_access = $2 WHERE char_id = $1",
		charID, now)
	if err != nil {
		return fmt.Errorf("failed to update last access: %w", err)
	}
	return nil
}

// UpdateStats updates character HP/MP/CP
func (r *CharacterRepositoryImpl) UpdateStats(ctx context.Context, charID int32, hp, mp, cp float64) error {
	_, err := r.db.Exec(ctx,
		"UPDATE characters SET cur_hp = $2, cur_mp = $3, cur_cp = $4 WHERE char_id = $1",
		charID, hp, mp, cp)
	if err != nil {
		return fmt.Errorf("failed to update stats: %w", err)
	}
	return nil
}

// UpdateExperience updates character experience and SP
func (r *CharacterRepositoryImpl) UpdateExperience(ctx context.Context, charID int32, exp int64, sp int) error {
	_, err := r.db.Exec(ctx,
		"UPDATE characters SET exp = $2, sp = $3 WHERE char_id = $1",
		charID, exp, sp)
	if err != nil {
		return fmt.Errorf("failed to update experience: %w", err)
	}
	return nil
}

// UpdateKarma updates character karma
func (r *CharacterRepositoryImpl) UpdateKarma(ctx context.Context, charID int32, karma int) error {
	_, err := r.db.Exec(ctx,
		"UPDATE characters SET karma = $2 WHERE char_id = $1",
		charID, karma)
	if err != nil {
		return fmt.Errorf("failed to update karma: %w", err)
	}
	return nil
}
