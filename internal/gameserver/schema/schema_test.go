package schema

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TestMigrations tests the database schema migrations
func TestMigrations(t *testing.T) {
	// Skip integration tests if no database URL provided
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}

	// Set log level to reduce noise during testing
	zerolog.SetGlobalLevel(zerolog.WarnLevel)

	ctx := context.Background()

	// Connect to test database
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "Failed to connect to test database")
	defer db.Close()

	// Clean up any existing test data
	t.Cleanup(func() {
		cleanupTestData(ctx, db, t)
	})

	// Test migration manager creation
	migrationManager := NewMigrationManager(db)
	require.NotNil(t, migrationManager, "Migration manager should not be nil")

	// Test getting migrations
	migrations, err := migrationManager.GetMigrations()
	require.NoError(t, err, "Should be able to get migrations")
	require.Greater(t, len(migrations), 0, "Should have at least one migration")

	// Verify migration order
	for i := 1; i < len(migrations); i++ {
		require.Greater(t, migrations[i].Version, migrations[i-1].Version,
			"Migrations should be ordered by version")
	}

	// Test migration status before applying
	err = migrationManager.GetStatus(ctx)
	require.NoError(t, err, "Should be able to get migration status")

	// Apply all migrations
	err = migrationManager.Migrate(ctx)
	require.NoError(t, err, "Should be able to apply all migrations")

	// Test migration status after applying
	err = migrationManager.GetStatus(ctx)
	require.NoError(t, err, "Should be able to get migration status after applying")

	// Run migrations again (should be no-op)
	err = migrationManager.Migrate(ctx)
	require.NoError(t, err, "Should be able to run migrations again without errors")

	// Validate table structure
	validateTableStructure(ctx, db, t)

	// Validate indexes
	validateIndexes(ctx, db, t)
}

// validateTableStructure verifies that all expected tables exist with correct columns
func validateTableStructure(ctx context.Context, db *pgxpool.Pool, t *testing.T) {
	// Check schema_migrations table
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'schema_migrations'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "schema_migrations table should exist")

	// Check characters table
	err = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'characters'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "characters table should exist")

	// Check character_items table
	err = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'character_items'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "character_items table should exist")

	// Check character_skills table
	err = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'character_skills'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "character_skills table should exist")

	// Check character_shortcuts table
	err = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'character_shortcuts'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "character_shortcuts table should exist")

	// Verify foreign key relationships
	var fkCount int
	err = db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY'
		AND tc.table_schema = 'public'
		AND kcu.table_name IN ('character_items', 'character_skills', 'character_shortcuts', 'character_macros')
	`).Scan(&fkCount)
	require.NoError(t, err)
	require.Greater(t, fkCount, 0, "Should have foreign key constraints")
}

// validateIndexes verifies that critical indexes exist
func validateIndexes(ctx context.Context, db *pgxpool.Pool, t *testing.T) {
	// Critical indexes that must exist for performance
	criticalIndexes := []string{
		"idx_characters_account_name",
		"idx_characters_char_name_unique",
		"idx_character_items_owner_id",
		"idx_character_items_owner_loc",
		"idx_character_skills_char_id",
		"idx_character_shortcuts_char_id",
	}

	for _, indexName := range criticalIndexes {
		var exists bool
		err := db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM pg_indexes 
				WHERE schemaname = 'public' 
				AND indexname = $1
			)
		`, indexName).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "Critical index %s should exist", indexName)
	}

	// Check that unique constraints work
	testUniqueConstraints(ctx, db, t)
}

// testUniqueConstraints verifies that unique constraints are enforced
func testUniqueConstraints(ctx context.Context, db *pgxpool.Pool, t *testing.T) {
	// Test character name uniqueness
	_, err := db.Exec(ctx, `
		INSERT INTO characters (account_name, char_name, race, class_id, base_class, x, y, z)
		VALUES ('testaccount', 'testchar', 0, 0, 0, 0, 0, 0)
	`)
	require.NoError(t, err, "Should be able to insert first character")

	// Try to insert duplicate character name (should fail)
	_, err = db.Exec(ctx, `
		INSERT INTO characters (account_name, char_name, race, class_id, base_class, x, y, z)
		VALUES ('testaccount2', 'testchar', 1, 1, 1, 100, 100, 100)
	`)
	require.Error(t, err, "Should not be able to insert duplicate character name")

	// Test account-slot uniqueness
	_, err = db.Exec(ctx, `
		INSERT INTO characters (account_name, char_name, race, class_id, base_class, x, y, z, char_slot)
		VALUES ('testaccount', 'testchar2', 0, 0, 0, 0, 0, 0, 0)
	`)
	require.NoError(t, err, "Should be able to insert character in slot 0")

	// Try to insert another character in same slot for same account (should fail)
	_, err = db.Exec(ctx, `
		INSERT INTO characters (account_name, char_name, race, class_id, base_class, x, y, z, char_slot)
		VALUES ('testaccount', 'testchar3', 1, 1, 1, 100, 100, 100, 0)
	`)
	require.Error(t, err, "Should not be able to insert duplicate slot for same account")
}

// cleanupTestData removes test data from database
func cleanupTestData(ctx context.Context, db *pgxpool.Pool, t *testing.T) {
	// Clean up in reverse order due to foreign key constraints
	tables := []string{
		"character_macro_commands",
		"character_macros", 
		"character_shortcuts",
		"character_skill_effects",
		"character_skills",
		"character_items",
		"characters",
		"schema_migrations",
	}

	for _, table := range tables {
		_, err := db.Exec(ctx, "DROP TABLE IF EXISTS "+table+" CASCADE")
		if err != nil {
			t.Logf("Warning: Failed to clean up table %s: %v", table, err)
		}
	}
}

// BenchmarkMigrations benchmarks migration performance
func BenchmarkMigrations(b *testing.B) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		b.Skip("TEST_DATABASE_URL not set, skipping benchmark")
	}

	ctx := context.Background()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		// Setup
		db, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			b.Fatal(err)
		}
		
		// Clean state
		cleanupTestData(ctx, db, &testing.T{})
		
		migrationManager := NewMigrationManager(db)
		
		b.StartTimer()
		
		// Benchmark migration application
		err = migrationManager.Migrate(ctx)
		if err != nil {
			b.Fatal(err)
		}
		
		b.StopTimer()
		db.Close()
	}
}