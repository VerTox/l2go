-- Migration 009: Relax character_shortcuts.level check to allow 0
-- The client sends level=0 for non-skill shortcuts (items, actions, macros —
-- RequestShortCutReg carries a level field that is only meaningful for skills).
-- The original CHECK (level >= 1) rejected every item shortcut, so quick-bar
-- placements could never persist. L2J stores level as-is (0 for items). (l2go-znj)

ALTER TABLE character_shortcuts DROP CONSTRAINT IF EXISTS character_shortcuts_level_check;
ALTER TABLE character_shortcuts ADD CONSTRAINT character_shortcuts_level_check CHECK (level >= 0);
