-- Migration 005: Add base stats columns to characters table
-- These columns store the per-class base stats (STR/DEX/CON/INT/WIT/MEN)
-- set from character templates during character creation.

ALTER TABLE characters
    ADD COLUMN IF NOT EXISTS base_str INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_dex INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_con INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_int INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_wit INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS base_men INTEGER NOT NULL DEFAULT 0;
