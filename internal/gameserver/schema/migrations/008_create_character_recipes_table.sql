-- Migration: Create character_recipes table
-- Version: 008
-- Description: Per-character recipe book (dwarven creation vs common recipes),
--              mirroring L2J's character_recipebook table. A recipe scroll item
--              (handler="Recipes") registers a recipe here on use and is consumed.

-- Character recipe book table.
--   recipe_id  : internal recipe-list id (recipes.xml <item id="..">), the L2J
--                registration key (NOT the scroll item id, NOT the crafted item id).
--   is_dwarven : true  -> dwarven creation recipe book (Create Dwarven Item)
--                false -> common recipe book (Create Common Item)
--   class_index: main/sub class slot (0=main, 1-3=subs), like character_skills.
CREATE TABLE character_recipes (
    char_id       INTEGER   NOT NULL REFERENCES characters(char_id) ON DELETE CASCADE,
    recipe_id     INTEGER   NOT NULL,
    is_dwarven    BOOLEAN   NOT NULL DEFAULT FALSE,
    class_index   INTEGER   NOT NULL DEFAULT 0,
    registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (char_id, recipe_id, class_index),
    CONSTRAINT character_recipes_class_index_check CHECK (class_index >= 0 AND class_index <= 3)
);

-- Load all recipes for a character (most frequent: recipe book open / login).
CREATE INDEX idx_character_recipes_char_id ON character_recipes(char_id);

-- Count / list recipes of one book (dwarven vs common) for the per-book limit check.
CREATE INDEX idx_character_recipes_book ON character_recipes(char_id, is_dwarven);

COMMENT ON TABLE character_recipes IS 'Per-character registered recipes (dwarven creation vs common), L2J character_recipebook equivalent';
COMMENT ON COLUMN character_recipes.recipe_id IS 'Internal recipe-list id (recipes.xml item id), registration key';
COMMENT ON COLUMN character_recipes.is_dwarven IS 'true = dwarven creation recipe, false = common recipe';
COMMENT ON COLUMN character_recipes.class_index IS 'Class index for multi-class system (0=main, 1-3=subs)';
