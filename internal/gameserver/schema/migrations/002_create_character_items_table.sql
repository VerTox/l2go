-- Migration: Create character_items table with performance indexes
-- Version: 002
-- Description: Character inventory and equipment items following L2J schema structure

-- Character items table (equipment + inventory)
CREATE TABLE character_items (
    object_id SERIAL PRIMARY KEY,
    owner_id INTEGER NOT NULL REFERENCES characters(char_id) ON DELETE CASCADE,
    item_id INTEGER NOT NULL,
    count BIGINT NOT NULL DEFAULT 1,
    loc VARCHAR(10) NOT NULL DEFAULT 'INVENTORY',
    loc_data INTEGER NOT NULL DEFAULT -1,
    enchant_level INTEGER DEFAULT 0 NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    
    -- Additional L2J compatible fields
    custom_type1 INTEGER DEFAULT 0 NOT NULL,
    custom_type2 INTEGER DEFAULT 0 NOT NULL,
    mana_left INTEGER DEFAULT -1 NOT NULL,
    time INTEGER DEFAULT 0 NOT NULL,
    
    -- Augmentation system
    augmentation_id INTEGER DEFAULT 0 NOT NULL,
    augmentation_skill1 INTEGER DEFAULT 0 NOT NULL,
    augmentation_skill2 INTEGER DEFAULT 0 NOT NULL,
    
    -- Item attributes and special properties  
    attribute_fire INTEGER DEFAULT 0 NOT NULL,
    attribute_water INTEGER DEFAULT 0 NOT NULL,
    attribute_wind INTEGER DEFAULT 0 NOT NULL,
    attribute_earth INTEGER DEFAULT 0 NOT NULL,
    attribute_holy INTEGER DEFAULT 0 NOT NULL,
    attribute_dark INTEGER DEFAULT 0 NOT NULL,
    
    -- Item visual effects
    visual_id INTEGER DEFAULT 0 NOT NULL,
    
    -- Blessed/Cursed status
    is_blessed BOOLEAN DEFAULT FALSE NOT NULL,
    is_protected BOOLEAN DEFAULT FALSE NOT NULL,
    
    -- Constraints
    CONSTRAINT character_items_count_check CHECK (count > 0),
    CONSTRAINT character_items_enchant_check CHECK (enchant_level >= 0 AND enchant_level <= 127),
    CONSTRAINT character_items_loc_check CHECK (loc IN ('INVENTORY', 'PAPERDOLL', 'WAREHOUSE', 'CLAN_WH', 'PET', 'PET_EQUIP', 'FREIGHT')),
    CONSTRAINT character_items_loc_data_check CHECK (
        (loc = 'PAPERDOLL' AND loc_data >= 0 AND loc_data <= 25) OR
        (loc != 'PAPERDOLL' AND loc_data = -1)
    ),
    CONSTRAINT character_items_attributes_check CHECK (
        attribute_fire >= 0 AND attribute_water >= 0 AND attribute_wind >= 0 AND
        attribute_earth >= 0 AND attribute_holy >= 0 AND attribute_dark >= 0
    )
);

-- Performance-critical indexes (ordered by query frequency)

-- 1. Owner-based queries (most frequent - load all character items)
CREATE INDEX idx_character_items_owner_id ON character_items(owner_id);

-- 2. Location-based queries (inventory, equipment, warehouse)
CREATE INDEX idx_character_items_owner_loc ON character_items(owner_id, loc);

-- 3. Paperdoll/Equipment slot queries (equipment display, stat calculations)
CREATE INDEX idx_character_items_paperdoll ON character_items(owner_id, loc_data) 
    WHERE loc = 'PAPERDOLL';

-- 4. Item type queries (for item searches, market analysis)
CREATE INDEX idx_character_items_item_id ON character_items(item_id);

-- 5. Enchanted items queries (for special effects, value calculations)
CREATE INDEX idx_character_items_enchanted ON character_items(enchant_level, item_id) 
    WHERE enchant_level > 0;

-- 6. Augmented items queries (for augmentation system)
CREATE INDEX idx_character_items_augmented ON character_items(augmentation_id) 
    WHERE augmentation_id > 0;

-- 7. Attributed items queries (elemental weapons/armor)
CREATE INDEX idx_character_items_attributed ON character_items(owner_id) 
    WHERE (attribute_fire > 0 OR attribute_water > 0 OR attribute_wind > 0 OR 
           attribute_earth > 0 OR attribute_holy > 0 OR attribute_dark > 0);

-- 8. Blessed/Protected items queries (for special item handling)
CREATE INDEX idx_character_items_blessed ON character_items(owner_id, is_blessed) 
    WHERE is_blessed = TRUE;

-- 9. Time-based items queries (temporary items, expiration)
CREATE INDEX idx_character_items_expirable ON character_items(time) 
    WHERE time > 0;

-- 10. Warehouse queries by location
CREATE INDEX idx_character_items_warehouse ON character_items(owner_id, loc, loc_data) 
    WHERE loc IN ('WAREHOUSE', 'CLAN_WH', 'FREIGHT');

-- 11. Visual items queries (for appearance system)
CREATE INDEX idx_character_items_visual ON character_items(visual_id) 
    WHERE visual_id > 0;

-- 12. Creation time queries (for item history, duplication detection)
CREATE INDEX idx_character_items_created_at ON character_items(created_at);

-- 13. Composite index for complete item location (fast exact lookups)
CREATE INDEX idx_character_items_location_composite ON character_items(owner_id, loc, loc_data, item_id);

-- Unique constraints
-- Ensure no duplicate paperdoll slots per character
CREATE UNIQUE INDEX idx_character_items_paperdoll_unique 
    ON character_items(owner_id, loc_data) 
    WHERE loc = 'PAPERDOLL' AND loc_data >= 0;

-- Comments for index usage
COMMENT ON INDEX idx_character_items_owner_id IS 'Primary query: load all items for character';
COMMENT ON INDEX idx_character_items_owner_loc IS 'Query items by location (inventory, equipment, etc)';
COMMENT ON INDEX idx_character_items_paperdoll IS 'Equipment slot queries, partial index for efficiency';
COMMENT ON INDEX idx_character_items_item_id IS 'Find all instances of specific item type';
COMMENT ON INDEX idx_character_items_enchanted IS 'High-value enchanted items queries';
COMMENT ON INDEX idx_character_items_paperdoll_unique IS 'Prevent duplicate equipment in same slot';

-- Table comments
COMMENT ON TABLE character_items IS 'Character inventory, equipment, and warehouse items with L2J compatibility';
COMMENT ON COLUMN character_items.object_id IS 'Unique item instance identifier';
COMMENT ON COLUMN character_items.owner_id IS 'Character who owns this item';
COMMENT ON COLUMN character_items.item_id IS 'Item template ID from game data';
COMMENT ON COLUMN character_items.count IS 'Stack count for stackable items';
COMMENT ON COLUMN character_items.loc IS 'Item location: INVENTORY, PAPERDOLL, WAREHOUSE, etc';
COMMENT ON COLUMN character_items.loc_data IS 'Location-specific data: paperdoll slot (-1 for non-paperdoll)';
COMMENT ON COLUMN character_items.enchant_level IS 'Enchantment level (0-127)';
COMMENT ON COLUMN character_items.augmentation_id IS 'Augmentation system identifier';
COMMENT ON COLUMN character_items.mana_left IS 'Remaining mana for consumable items';
COMMENT ON COLUMN character_items.time IS 'Expiration time for temporary items (0 = permanent)';
COMMENT ON COLUMN character_items.visual_id IS 'Visual transformation item ID';

-- Paperdoll slot reference (for documentation)
/* Paperdoll Slots (loc_data values when loc = 'PAPERDOLL'):
   0=under, 1=rear, 2=lear, 3=neck, 4=rfinger, 5=lfinger, 6=head, 7=rhand, 8=lhand,
   9=gloves, 10=chest, 11=legs, 12=feet, 13=back, 14=lrhand, 15=hair, 16=hair2, 
   17=rbracelet, 18=lbracelet, 19=deco1, 20=deco2, 21=deco3, 22=deco4, 23=deco5, 
   24=deco6, 25=belt
*/