# GameServer Database Schema

PostgreSQL migrations for L2Go GameServer, auto-applied on startup.

## Migrations

| File | Description |
|------|-------------|
| 001_create_characters_table.sql | Character data (14 indexes) |
| 002_create_character_items_table.sql | Items/inventory (13 indexes) |
| 003_create_character_skills_table.sql | Skills and effects (5 indexes) |
| 004_create_character_shortcuts_table.sql | UI shortcuts (4 indexes) |
| 005_create_item_templates_table.sql | Item definitions |
| 006_create_spawnlist_table.sql | NPC spawn data (~38K entries) |

**Total**: 36+ indexes (15 partial), 5 unique constraints.

## Usage

```go
migrationManager := schema.NewMigrationManager(db)
err := migrationManager.Migrate(ctx)
```

Migrations are embedded via Go embed and run automatically on GameServer startup. Add new migrations as `00X_description.sql` — never edit existing ones.

## Requirements

- PostgreSQL 12+ (recommended: 14+)
- pgx/v5 driver with connection pooling
