-- Migration: Create characters table with optimized indexes
-- Version: 001
-- Description: Core character data table following L2J schema structure

-- Characters table (core entity)
CREATE TABLE characters (
    char_id SERIAL PRIMARY KEY,
    account_name VARCHAR(45) NOT NULL,
    char_name VARCHAR(35) NOT NULL UNIQUE,
    level INTEGER DEFAULT 1 NOT NULL,
    max_hp INTEGER DEFAULT 100 NOT NULL,
    cur_hp DOUBLE PRECISION DEFAULT 100 NOT NULL,
    max_mp INTEGER DEFAULT 50 NOT NULL,
    cur_mp DOUBLE PRECISION DEFAULT 50 NOT NULL,
    max_cp INTEGER DEFAULT 0 NOT NULL,
    cur_cp INTEGER DEFAULT 0 NOT NULL,
    face INTEGER DEFAULT 0 NOT NULL,
    hair_style INTEGER DEFAULT 0 NOT NULL,
    hair_color INTEGER DEFAULT 0 NOT NULL,
    sex INTEGER DEFAULT 0 NOT NULL,
    exp BIGINT DEFAULT 0 NOT NULL,
    sp INTEGER DEFAULT 0 NOT NULL,
    karma INTEGER DEFAULT 0 NOT NULL,
    pk_kills INTEGER DEFAULT 0 NOT NULL,
    pvp_kills INTEGER DEFAULT 0 NOT NULL,
    clan_id INTEGER DEFAULT 0 NOT NULL,
    race INTEGER NOT NULL,
    class_id INTEGER NOT NULL,
    base_class INTEGER NOT NULL,
    delete_time BIGINT DEFAULT 0 NOT NULL,
    vitality_points INTEGER DEFAULT 2000 NOT NULL,
    access_level INTEGER DEFAULT 0 NOT NULL,
    x INTEGER NOT NULL,
    y INTEGER NOT NULL,
    z INTEGER NOT NULL,
    heading INTEGER DEFAULT 0 NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    last_access BIGINT DEFAULT 0 NOT NULL,
    
    -- Additional L2J compatible fields
    online_time INTEGER DEFAULT 0 NOT NULL,
    online_status BOOLEAN DEFAULT FALSE NOT NULL,
    char_slot INTEGER DEFAULT 0 NOT NULL,
    newbie BOOLEAN DEFAULT TRUE NOT NULL,
    noble BOOLEAN DEFAULT FALSE NOT NULL,
    hero BOOLEAN DEFAULT FALSE NOT NULL,
    hero_end_date TIMESTAMP NULL,
    
    -- PvP/PK punishment system
    death_penalty_level INTEGER DEFAULT 0 NOT NULL,
    
    -- Title and recommendation system
    title VARCHAR(255) DEFAULT '' NOT NULL,
    rec_have INTEGER DEFAULT 0 NOT NULL,
    rec_left INTEGER DEFAULT 0 NOT NULL,
    
    -- Fame and fishing points
    fame INTEGER DEFAULT 0 NOT NULL,
    fishing_points INTEGER DEFAULT 0 NOT NULL,
    
    -- Constraints
    CONSTRAINT characters_level_check CHECK (level >= 1 AND level <= 85),
    CONSTRAINT characters_sex_check CHECK (sex IN (0, 1)),
    CONSTRAINT characters_race_check CHECK (race >= 0 AND race <= 5),
    CONSTRAINT characters_access_level_check CHECK (access_level >= 0),
    CONSTRAINT characters_char_slot_check CHECK (char_slot >= 0 AND char_slot <= 6)
);

-- Critical indexes for performance (ordered by usage frequency)

-- 1. Account-based character queries (most frequent)
CREATE INDEX idx_characters_account_name ON characters(account_name);

-- 2. Character name uniqueness and lookups
CREATE UNIQUE INDEX idx_characters_char_name_unique ON characters(char_name);

-- 3. Online status queries
CREATE INDEX idx_characters_online_status ON characters(online_status) WHERE online_status = TRUE;

-- 4. Deletion timer queries (partial index for efficiency)
CREATE INDEX idx_characters_delete_time ON characters(delete_time) WHERE delete_time > 0;

-- 5. Level-based queries (for level restrictions, PvP matching, etc.)
CREATE INDEX idx_characters_level ON characters(level);

-- 6. Position-based queries (for world loading, teleportation)
CREATE INDEX idx_characters_position ON characters(x, y, z);

-- 7. Clan-based queries (partial index for clan members only)
CREATE INDEX idx_characters_clan_id ON characters(clan_id) WHERE clan_id > 0;

-- 8. Last access queries (for inactive player cleanup)
CREATE INDEX idx_characters_last_access ON characters(last_access);

-- 9. Character creation time queries (for statistics, analytics)
CREATE INDEX idx_characters_created_at ON characters(created_at);

-- 10. Class-based queries (for class restrictions, balancing)
CREATE INDEX idx_characters_class_id ON characters(class_id);

-- 11. Access level queries (for GM tools, admin functions)
CREATE INDEX idx_characters_access_level ON characters(access_level) WHERE access_level > 0;

-- 12. Hero status queries (partial index for heroes only)
CREATE INDEX idx_characters_hero_status ON characters(hero) WHERE hero = TRUE;

-- 13. PK/PvP queries (for karma system, PvP rankings)
CREATE INDEX idx_characters_pk_kills ON characters(pk_kills) WHERE pk_kills > 0;
CREATE INDEX idx_characters_pvp_kills ON characters(pvp_kills) WHERE pvp_kills > 0;

-- 14. Composite index for character slot management per account
CREATE UNIQUE INDEX idx_characters_account_slot ON characters(account_name, char_slot);

-- Comments for index usage
COMMENT ON INDEX idx_characters_account_name IS 'Primary query: load characters by account';
COMMENT ON INDEX idx_characters_char_name_unique IS 'Uniqueness constraint and name lookups';
COMMENT ON INDEX idx_characters_online_status IS 'Track online players, partial index for efficiency';
COMMENT ON INDEX idx_characters_delete_time IS 'Find characters pending deletion, partial index';
COMMENT ON INDEX idx_characters_position IS 'World position queries, teleportation, area loading';
COMMENT ON INDEX idx_characters_clan_id IS 'Clan member queries, partial index for clan members only';
COMMENT ON INDEX idx_characters_account_slot IS 'Ensure unique slot per account, prevent slot conflicts';

-- Table comments
COMMENT ON TABLE characters IS 'Core character data following L2J schema with performance optimizations';
COMMENT ON COLUMN characters.char_id IS 'Unique character identifier, auto-incrementing';
COMMENT ON COLUMN characters.account_name IS 'Associated account name from login server';
COMMENT ON COLUMN characters.char_name IS 'Character display name, must be unique across server';
COMMENT ON COLUMN characters.delete_time IS 'Unix timestamp when character will be deleted, 0 = not marked for deletion';
COMMENT ON COLUMN characters.online_status IS 'Current online status for duplicate login prevention';
COMMENT ON COLUMN characters.char_slot IS 'Character slot position (0-6) within account';
COMMENT ON COLUMN characters.heading IS 'Character facing direction (0-65535)';
COMMENT ON COLUMN characters.last_access IS 'Unix timestamp of last login for cleanup/statistics';