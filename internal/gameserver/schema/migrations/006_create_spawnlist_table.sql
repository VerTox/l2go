-- Migration 006: Create spawnlist table for NPC spawns
-- Mirrors L2J spawnlist table structure adapted for PostgreSQL.
-- Populated on first startup from L2J datapack SQL file.

CREATE TABLE IF NOT EXISTS spawnlist (
    id SERIAL PRIMARY KEY,
    location VARCHAR(40) NOT NULL DEFAULT 'unset',
    count SMALLINT NOT NULL DEFAULT 1,
    npc_templateid INTEGER NOT NULL DEFAULT 0,
    locx INTEGER NOT NULL DEFAULT 0,
    locy INTEGER NOT NULL DEFAULT 0,
    locz INTEGER NOT NULL DEFAULT 0,
    heading INTEGER NOT NULL DEFAULT 0,
    respawn_delay INTEGER NOT NULL DEFAULT 0,
    loc_id INTEGER NOT NULL DEFAULT 0,
    period_of_day SMALLINT NOT NULL DEFAULT 0
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_spawnlist_npc_templateid ON spawnlist (npc_templateid);
CREATE INDEX IF NOT EXISTS idx_spawnlist_loc ON spawnlist (locx, locy, locz);
