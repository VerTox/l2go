package schema

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// MigrationManager handles database schema migrations
type MigrationManager struct {
	db *pgxpool.Pool
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *pgxpool.Pool) *MigrationManager {
	return &MigrationManager{db: db}
}

// GetMigrations returns all available migrations sorted by version
func (m *MigrationManager) GetMigrations() ([]Migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Parse migration version from filename (e.g., "001_create_characters_table.sql")
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Warn().Str("file", entry.Name()).Msg("Invalid migration filename format")
			continue
		}

		content, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read migration %s: %w", entry.Name(), err)
		}

		name := strings.TrimSuffix(strings.Join(parts[1:], "_"), ".sql")
		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			SQL:     string(content),
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// createMigrationsTable creates the migrations tracking table if it doesn't exist
func (m *MigrationManager) createMigrationsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := m.db.Exec(ctx, query)
	return err
}

// getAppliedMigrations returns the list of already applied migration versions
func (m *MigrationManager) getAppliedMigrations(ctx context.Context) (map[int]bool, error) {
	query := "SELECT version FROM schema_migrations"
	rows, err := m.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, nil
}

// recordMigration records that a migration has been applied
func (m *MigrationManager) recordMigration(ctx context.Context, migration Migration) error {
	query := "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)"
	_, err := m.db.Exec(ctx, query, migration.Version, migration.Name)
	return err
}

// Migrate runs all pending migrations
func (m *MigrationManager) Migrate(ctx context.Context) error {
	// Create migrations table if it doesn't exist
	if err := m.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get all available migrations
	migrations, err := m.GetMigrations()
	if err != nil {
		return fmt.Errorf("failed to get migrations: %w", err)
	}

	// Get already applied migrations
	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Apply pending migrations
	for _, migration := range migrations {
		if applied[migration.Version] {
			log.Debug().Int("version", migration.Version).Str("name", migration.Name).
				Msg("Migration already applied, skipping")
			continue
		}

		log.Info().Int("version", migration.Version).Str("name", migration.Name).
			Msg("Applying migration")

		// Execute migration in a transaction
		tx, err := m.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		// Execute migration SQL
		if _, err := tx.Exec(ctx, migration.SQL); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
		}

		// Record migration as applied
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)",
			migration.Version, migration.Name); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		log.Info().Int("version", migration.Version).Str("name", migration.Name).
			Msg("Migration applied successfully")
	}

	log.Info().Int("total", len(migrations)).Msg("All migrations completed")
	return nil
}

// GetStatus returns the current migration status
func (m *MigrationManager) GetStatus(ctx context.Context) error {
	// Create migrations table if it doesn't exist (for status check)
	if err := m.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	migrations, err := m.GetMigrations()
	if err != nil {
		return fmt.Errorf("failed to get migrations: %w", err)
	}

	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	log.Info().Msg("Migration Status:")
	for _, migration := range migrations {
		status := "PENDING"
		if applied[migration.Version] {
			status = "APPLIED"
		}
		log.Info().Int("version", migration.Version).Str("name", migration.Name).
			Str("status", status).Msg("Migration")
	}

	return nil
}