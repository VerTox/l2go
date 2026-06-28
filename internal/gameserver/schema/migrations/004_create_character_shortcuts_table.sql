-- Migration: Create character_shortcuts table
-- Version: 004  
-- Description: Character UI shortcuts and macros following L2J schema structure

-- Character shortcuts/macros table
CREATE TABLE character_shortcuts (
    char_id INTEGER NOT NULL REFERENCES characters(char_id) ON DELETE CASCADE,
    slot INTEGER NOT NULL,
    page INTEGER NOT NULL DEFAULT 0,
    type INTEGER NOT NULL,
    shortcut_id INTEGER NOT NULL,
    level INTEGER DEFAULT 1 NOT NULL,
    sub_level INTEGER DEFAULT 0 NOT NULL,
    
    -- Additional shortcut metadata
    class_index INTEGER DEFAULT 0 NOT NULL,
    
    PRIMARY KEY (char_id, slot, page, class_index),
    
    -- Constraints
    CONSTRAINT character_shortcuts_slot_check CHECK (slot >= 0 AND slot <= 143),
    CONSTRAINT character_shortcuts_page_check CHECK (page >= 0 AND page <= 23),
    CONSTRAINT character_shortcuts_type_check CHECK (type >= 0 AND type <= 6),
    CONSTRAINT character_shortcuts_level_check CHECK (level >= 1),
    CONSTRAINT character_shortcuts_class_index_check CHECK (class_index >= 0 AND class_index <= 3)
);

-- Performance indexes

-- 1. Character-based shortcut queries (load all shortcuts for character)
CREATE INDEX idx_character_shortcuts_char_id ON character_shortcuts(char_id);

-- 2. Page-based queries (load specific shortcut page)
CREATE INDEX idx_character_shortcuts_char_page ON character_shortcuts(char_id, page, class_index);

-- 3. Type-based queries (find all shortcuts of specific type)
CREATE INDEX idx_character_shortcuts_type ON character_shortcuts(char_id, type);

-- 4. Class-specific shortcuts (for dual class system)
CREATE INDEX idx_character_shortcuts_class_index ON character_shortcuts(char_id, class_index);

-- Comments
COMMENT ON INDEX idx_character_shortcuts_char_id IS 'Primary query: load all shortcuts for character';
COMMENT ON INDEX idx_character_shortcuts_char_page IS 'Load specific shortcut page for UI';
COMMENT ON INDEX idx_character_shortcuts_type IS 'Query shortcuts by type (item, skill, action, etc)';

-- Table comments
COMMENT ON TABLE character_shortcuts IS 'Character UI shortcuts and macros with L2J compatibility';
COMMENT ON COLUMN character_shortcuts.char_id IS 'Character who owns these shortcuts';
COMMENT ON COLUMN character_shortcuts.slot IS 'Shortcut slot position (0-143)';
COMMENT ON COLUMN character_shortcuts.page IS 'Shortcut page number (0-23)';
COMMENT ON COLUMN character_shortcuts.type IS 'Shortcut type: 0=none, 1=item, 2=skill, 3=action, 4=macro, 5=recipe, 6=bookmark';
COMMENT ON COLUMN character_shortcuts.shortcut_id IS 'ID of the target (item_id, skill_id, action_id, etc)';
COMMENT ON COLUMN character_shortcuts.level IS 'Level for skills, quantity for items';
COMMENT ON COLUMN character_shortcuts.sub_level IS 'Sub-level for enchanted skills';
COMMENT ON COLUMN character_shortcuts.class_index IS 'Class index for multi-class system (0=main, 1-3=subs)';

-- Character macros table (for complex macro commands)
CREATE TABLE character_macros (
    char_id INTEGER NOT NULL REFERENCES characters(char_id) ON DELETE CASCADE,
    macro_id INTEGER NOT NULL,
    icon INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(40) NOT NULL DEFAULT '',
    descr VARCHAR(80) NOT NULL DEFAULT '',
    acronym VARCHAR(4) NOT NULL DEFAULT '',
    
    PRIMARY KEY (char_id, macro_id),
    
    -- Constraints
    CONSTRAINT character_macros_macro_id_check CHECK (macro_id >= 1 AND macro_id <= 48),
    CONSTRAINT character_macros_icon_check CHECK (icon >= 0)
);

-- Character macro commands table (macro command sequences)
CREATE TABLE character_macro_commands (
    char_id INTEGER NOT NULL,
    macro_id INTEGER NOT NULL,
    command_id INTEGER NOT NULL,
    type INTEGER NOT NULL DEFAULT 0,
    d1 INTEGER NOT NULL DEFAULT 0,
    d2 INTEGER NOT NULL DEFAULT 0,
    cmd VARCHAR(80) NOT NULL DEFAULT '',
    
    PRIMARY KEY (char_id, macro_id, command_id),
    FOREIGN KEY (char_id, macro_id) REFERENCES character_macros(char_id, macro_id) ON DELETE CASCADE,
    
    -- Constraints  
    CONSTRAINT character_macro_commands_command_id_check CHECK (command_id >= 1 AND command_id <= 12),
    CONSTRAINT character_macro_commands_type_check CHECK (type >= 0 AND type <= 6)
);

-- Performance indexes for macros
CREATE INDEX idx_character_macros_char_id ON character_macros(char_id);
CREATE INDEX idx_character_macro_commands_char_macro ON character_macro_commands(char_id, macro_id);

-- Comments for macros
COMMENT ON TABLE character_macros IS 'Character macro definitions';
COMMENT ON TABLE character_macro_commands IS 'Individual commands within character macros';
COMMENT ON COLUMN character_macros.macro_id IS 'Unique macro identifier per character (1-48)';
COMMENT ON COLUMN character_macros.icon IS 'Macro icon ID for UI display';
COMMENT ON COLUMN character_macros.name IS 'Macro display name';
COMMENT ON COLUMN character_macros.descr IS 'Macro description';
COMMENT ON COLUMN character_macros.acronym IS 'Macro short name/acronym';
COMMENT ON COLUMN character_macro_commands.command_id IS 'Command sequence order (1-12)';
COMMENT ON COLUMN character_macro_commands.type IS 'Command type: 0=none, 1=skill, 2=action, 3=chat, 4=item, 5=delay, 6=shortcut';
COMMENT ON COLUMN character_macro_commands.cmd IS 'Command text or parameters';