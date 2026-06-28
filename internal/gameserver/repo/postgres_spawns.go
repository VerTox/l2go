package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// SpawnRepositoryImpl implements SpawnRepository using PostgreSQL
type SpawnRepositoryImpl struct {
	db *pgxpool.Pool
}

// NewSpawnRepository creates a new spawn repository
func NewSpawnRepository(db *pgxpool.Pool) *SpawnRepositoryImpl {
	return &SpawnRepositoryImpl{db: db}
}

// GetAll returns all spawn entries from the database
func (r *SpawnRepositoryImpl) GetAll(ctx context.Context) ([]models.SpawnData, error) {
	query := `SELECT npc_templateid, locx, locy, locz, heading, respawn_delay, count
		FROM spawnlist ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query spawnlist: %w", err)
	}
	defer rows.Close()

	var spawns []models.SpawnData
	for rows.Next() {
		var sd models.SpawnData
		if err := rows.Scan(&sd.NpcID, &sd.X, &sd.Y, &sd.Z, &sd.Heading, &sd.RespawnDelay, &sd.Count); err != nil {
			return nil, fmt.Errorf("failed to scan spawn row: %w", err)
		}
		spawns = append(spawns, sd)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating spawn rows: %w", err)
	}

	return spawns, nil
}

// GetCount returns the total number of spawn entries
func (r *SpawnRepositoryImpl) GetCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM spawnlist`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count spawnlist: %w", err)
	}
	return count, nil
}

// BulkInsert inserts a batch of spawn entries using PostgreSQL COPY protocol for speed.
// Returns the number of rows inserted.
func (r *SpawnRepositoryImpl) BulkInsert(ctx context.Context, spawns []models.SpawnData) (int, error) {
	columns := []string{"npc_templateid", "locx", "locy", "locz", "heading", "respawn_delay", "count"}

	rows := make([][]interface{}, 0, len(spawns))
	for _, s := range spawns {
		rows = append(rows, []interface{}{
			s.NpcID,
			s.X,
			s.Y,
			s.Z,
			s.Heading,
			s.RespawnDelay,
			s.Count,
		})
	}

	copyCount, err := r.db.CopyFrom(
		ctx,
		pgx.Identifier{"spawnlist"},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to bulk insert spawns: %w", err)
	}

	return int(copyCount), nil
}
