-- Migration: Create character_skills table
-- Version: 003
-- Description: Character skills and abilities following L2J schema structure

-- Character skills table
CREATE TABLE character_skills (
    char_id INTEGER NOT NULL REFERENCES characters(char_id) ON DELETE CASCADE,
    skill_id INTEGER NOT NULL,
    skill_level INTEGER NOT NULL DEFAULT 1,
    
    -- Additional skill metadata
    class_index INTEGER DEFAULT 0 NOT NULL,
    learned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    
    -- Constraints
    PRIMARY KEY (char_id, skill_id, class_index),
    CONSTRAINT character_skills_level_check CHECK (skill_level >= 1 AND skill_level <= 99),
    CONSTRAINT character_skills_class_index_check CHECK (class_index >= 0 AND class_index <= 3),
    CONSTRAINT character_skills_unique_skill_per_class UNIQUE (char_id, skill_id, class_index)
);

-- Performance indexes

-- 1. Character-based skill queries (most frequent - load all character skills)
CREATE INDEX idx_character_skills_char_id ON character_skills(char_id);

-- 2. Skill type queries (find all characters with specific skill)
CREATE INDEX idx_character_skills_skill_id ON character_skills(skill_id);

-- 3. High-level skills queries (for skill requirements, prestige tracking)
CREATE INDEX idx_character_skills_high_level ON character_skills(skill_id, skill_level) 
    WHERE skill_level >= 10;

-- 4. Class-specific skills queries (for dual class system)
CREATE INDEX idx_character_skills_class_index ON character_skills(char_id, class_index);

-- 5. Composite index for skill lookups and updates
CREATE INDEX idx_character_skills_composite ON character_skills(char_id, skill_id, class_index, skill_level);

-- Comments
COMMENT ON INDEX idx_character_skills_char_id IS 'Primary query: load all skills for character';
COMMENT ON INDEX idx_character_skills_skill_id IS 'Find all characters with specific skill';
COMMENT ON INDEX idx_character_skills_high_level IS 'Track high-level skill achievements';
COMMENT ON INDEX idx_character_skills_class_index IS 'Dual/multi-class skill management';

-- Table comments
COMMENT ON TABLE character_skills IS 'Character learned skills and abilities with L2J compatibility';
COMMENT ON COLUMN character_skills.char_id IS 'Character who owns this skill';
COMMENT ON COLUMN character_skills.skill_id IS 'Skill template ID from game data';
COMMENT ON COLUMN character_skills.skill_level IS 'Current skill level (1-99)';
COMMENT ON COLUMN character_skills.class_index IS 'Class index for multi-class system (0=main, 1-3=subs)';
COMMENT ON COLUMN character_skills.learned_at IS 'Timestamp when skill was learned';

-- Character sub-skills table (for skill effects, enchantments)
CREATE TABLE character_skill_effects (
    char_id INTEGER NOT NULL REFERENCES characters(char_id) ON DELETE CASCADE,
    skill_id INTEGER NOT NULL,
    skill_level INTEGER NOT NULL,
    effect_index INTEGER NOT NULL DEFAULT 0,
    remaining_time INTEGER DEFAULT -1 NOT NULL,
    
    -- Effect metadata
    effect_count INTEGER DEFAULT 0 NOT NULL,
    
    PRIMARY KEY (char_id, skill_id, effect_index),
    CONSTRAINT character_skill_effects_level_check CHECK (skill_level >= 1),
    CONSTRAINT character_skill_effects_time_check CHECK (remaining_time >= -1)
);

-- Performance indexes for skill effects
CREATE INDEX idx_character_skill_effects_char_id ON character_skill_effects(char_id);
CREATE INDEX idx_character_skill_effects_expiring ON character_skill_effects(remaining_time) 
    WHERE remaining_time > 0;

-- Comments for skill effects
COMMENT ON TABLE character_skill_effects IS 'Active skill effects and buffs on characters';
COMMENT ON COLUMN character_skill_effects.remaining_time IS 'Effect duration in seconds (-1 = permanent, 0 = expired)';
COMMENT ON COLUMN character_skill_effects.effect_count IS 'Stacking count for stackable effects';