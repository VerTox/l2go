package schema

import (
	"database/sql"
	"embed"
	"fmt"

	migrate "github.com/rubenv/sql-migrate"

	_ "github.com/lib/pq"
)

//go:embed *.sql
var migrations embed.FS

func Up(dsn string) (int, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return 0, fmt.Errorf("sql open failed: %w (dsn %q)", err, dsn)
	}
	defer db.Close()

	m := migrate.EmbedFileSystemMigrationSource{
		FileSystem: migrations,
		Root:       ".",
	}

	return migrate.Exec(db, "postgres", m, migrate.Up)
}
