package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// ItemRepositoryImpl implements ItemRepository for PostgreSQL
type ItemRepositoryImpl struct {
	db pgxDB
}

// NewItemRepository creates an item repository with pool
func NewItemRepository(db pgxDB) *ItemRepositoryImpl {
	return &ItemRepositoryImpl{db: db}
}

// NewItemRepositoryTx creates an item repository with transaction
func NewItemRepositoryTx(tx pgx.Tx) *ItemRepositoryImpl {
	return &ItemRepositoryImpl{db: tx}
}

// GetByCharacter retrieves all items for a character
func (r *ItemRepositoryImpl) GetByCharacter(ctx context.Context, charID int32) ([]models.CharacterItem, error) {
	query := `
		SELECT object_id, owner_id, item_id, count, loc, loc_data, enchant_level, created_at,
			   custom_type1, custom_type2, mana_left, time, augmentation_id,
			   augmentation_skill1, augmentation_skill2, attribute_fire, attribute_water,
			   attribute_wind, attribute_earth, attribute_holy, attribute_dark,
			   visual_id, is_blessed, is_protected
		FROM character_items 
		WHERE owner_id = $1 
		ORDER BY loc, loc_data, object_id`

	rows, err := r.db.Query(ctx, query, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to query character items: %w", err)
	}
	defer rows.Close()

	var items []models.CharacterItem
	for rows.Next() {
		var item models.CharacterItem

		err := rows.Scan(
			&item.ObjectID, &item.OwnerID, &item.ItemID, &item.Count,
			&item.Loc, &item.LocData, &item.EnchantLevel, &item.CreatedAt,
			&item.CustomType1, &item.CustomType2, &item.ManaLeft, &item.Time,
			&item.AugmentationID, &item.AugmentationSkill1, &item.AugmentationSkill2,
			&item.AttributeFire, &item.AttributeWater, &item.AttributeWind,
			&item.AttributeEarth, &item.AttributeHoly, &item.AttributeDark,
			&item.VisualID, &item.IsBlessed, &item.IsProtected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

// GetByObjectID retrieves an item by object ID
func (r *ItemRepositoryImpl) GetByObjectID(ctx context.Context, objectID int32) (*models.CharacterItem, error) {
	query := `
		SELECT object_id, owner_id, item_id, count, loc, loc_data, enchant_level, created_at,
			   custom_type1, custom_type2, mana_left, time, augmentation_id,
			   augmentation_skill1, augmentation_skill2, attribute_fire, attribute_water,
			   attribute_wind, attribute_earth, attribute_holy, attribute_dark,
			   visual_id, is_blessed, is_protected
		FROM character_items 
		WHERE object_id = $1`

	var item models.CharacterItem

	err := r.db.QueryRow(ctx, query, objectID).Scan(
		&item.ObjectID, &item.OwnerID, &item.ItemID, &item.Count,
		&item.Loc, &item.LocData, &item.EnchantLevel, &item.CreatedAt,
		&item.CustomType1, &item.CustomType2, &item.ManaLeft, &item.Time,
		&item.AugmentationID, &item.AugmentationSkill1, &item.AugmentationSkill2,
		&item.AttributeFire, &item.AttributeWater, &item.AttributeWind,
		&item.AttributeEarth, &item.AttributeHoly, &item.AttributeDark,
		&item.VisualID, &item.IsBlessed, &item.IsProtected,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get item by object ID: %w", err)
	}

	return &item, nil
}

// Create creates a new item
func (r *ItemRepositoryImpl) Create(ctx context.Context, item *models.CharacterItem) error {
	query := `
		INSERT INTO character_items (
			owner_id, item_id, count, loc, loc_data, enchant_level,
			custom_type1, custom_type2, mana_left, time, augmentation_id,
			augmentation_skill1, augmentation_skill2, attribute_fire, attribute_water,
			attribute_wind, attribute_earth, attribute_holy, attribute_dark,
			visual_id, is_blessed, is_protected
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		) RETURNING object_id, created_at`

	err := r.db.QueryRow(ctx, query,
		item.OwnerID, item.ItemID, item.Count, item.Loc, item.LocData,
		item.EnchantLevel, item.CustomType1, item.CustomType2, item.ManaLeft,
		item.Time, item.AugmentationID, item.AugmentationSkill1,
		item.AugmentationSkill2, item.AttributeFire, item.AttributeWater,
		item.AttributeWind, item.AttributeEarth, item.AttributeHoly,
		item.AttributeDark, item.VisualID, item.IsBlessed, item.IsProtected,
	).Scan(&item.ObjectID, &item.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}

	return nil
}

// Update updates an existing item
func (r *ItemRepositoryImpl) Update(ctx context.Context, item *models.CharacterItem) error {
	query := `
		UPDATE character_items SET
			count = $2, loc = $3, loc_data = $4, enchant_level = $5,
			custom_type1 = $6, custom_type2 = $7, mana_left = $8, time = $9,
			augmentation_id = $10, augmentation_skill1 = $11, augmentation_skill2 = $12,
			attribute_fire = $13, attribute_water = $14, attribute_wind = $15,
			attribute_earth = $16, attribute_holy = $17, attribute_dark = $18,
			visual_id = $19, is_blessed = $20, is_protected = $21
		WHERE object_id = $1`

	_, err := r.db.Exec(ctx, query,
		item.ObjectID, item.Count, item.Loc, item.LocData, item.EnchantLevel,
		item.CustomType1, item.CustomType2, item.ManaLeft, item.Time,
		item.AugmentationID, item.AugmentationSkill1, item.AugmentationSkill2,
		item.AttributeFire, item.AttributeWater, item.AttributeWind,
		item.AttributeEarth, item.AttributeHoly, item.AttributeDark,
		item.VisualID, item.IsBlessed, item.IsProtected,
	)

	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	return nil
}

// Delete deletes an item by object ID
func (r *ItemRepositoryImpl) Delete(ctx context.Context, objectID int32) error {
	_, err := r.db.Exec(ctx, "DELETE FROM character_items WHERE object_id = $1", objectID)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}
	return nil
}

// DeleteByCharacter deletes all items for a character (used when character is deleted)
func (r *ItemRepositoryImpl) DeleteByCharacter(ctx context.Context, charID int32) error {
	_, err := r.db.Exec(ctx, "DELETE FROM character_items WHERE owner_id = $1", charID)
	if err != nil {
		return fmt.Errorf("failed to delete character items: %w", err)
	}
	return nil
}

// GetInventory retrieves items in inventory location
func (r *ItemRepositoryImpl) GetInventory(ctx context.Context, charID int32) ([]models.CharacterItem, error) {
	query := `
		SELECT object_id, owner_id, item_id, count, loc, loc_data, enchant_level, created_at,
			   custom_type1, custom_type2, mana_left, time, augmentation_id,
			   augmentation_skill1, augmentation_skill2, attribute_fire, attribute_water,
			   attribute_wind, attribute_earth, attribute_holy, attribute_dark,
			   visual_id, is_blessed, is_protected
		FROM character_items 
		WHERE owner_id = $1 AND loc = 'INVENTORY'
		ORDER BY object_id`

	rows, err := r.db.Query(ctx, query, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to query inventory items: %w", err)
	}
	defer rows.Close()

	var items []models.CharacterItem
	for rows.Next() {
		var item models.CharacterItem

		err := rows.Scan(
			&item.ObjectID, &item.OwnerID, &item.ItemID, &item.Count,
			&item.Loc, &item.LocData, &item.EnchantLevel, &item.CreatedAt,
			&item.CustomType1, &item.CustomType2, &item.ManaLeft, &item.Time,
			&item.AugmentationID, &item.AugmentationSkill1, &item.AugmentationSkill2,
			&item.AttributeFire, &item.AttributeWater, &item.AttributeWind,
			&item.AttributeEarth, &item.AttributeHoly, &item.AttributeDark,
			&item.VisualID, &item.IsBlessed, &item.IsProtected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory item: %w", err)
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

// GetPaperdoll retrieves equipped items (paperdoll)
func (r *ItemRepositoryImpl) GetPaperdoll(ctx context.Context, charID int32) ([]models.CharacterItem, error) {
	query := `
		SELECT object_id, owner_id, item_id, count, loc, loc_data, enchant_level, created_at,
			   custom_type1, custom_type2, mana_left, time, augmentation_id,
			   augmentation_skill1, augmentation_skill2, attribute_fire, attribute_water,
			   attribute_wind, attribute_earth, attribute_holy, attribute_dark,
			   visual_id, is_blessed, is_protected
		FROM character_items 
		WHERE owner_id = $1 AND loc = 'PAPERDOLL'
		ORDER BY loc_data`

	rows, err := r.db.Query(ctx, query, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to query paperdoll items: %w", err)
	}
	defer rows.Close()

	var items []models.CharacterItem
	for rows.Next() {
		var item models.CharacterItem

		err := rows.Scan(
			&item.ObjectID, &item.OwnerID, &item.ItemID, &item.Count,
			&item.Loc, &item.LocData, &item.EnchantLevel, &item.CreatedAt,
			&item.CustomType1, &item.CustomType2, &item.ManaLeft, &item.Time,
			&item.AugmentationID, &item.AugmentationSkill1, &item.AugmentationSkill2,
			&item.AttributeFire, &item.AttributeWater, &item.AttributeWind,
			&item.AttributeEarth, &item.AttributeHoly, &item.AttributeDark,
			&item.VisualID, &item.IsBlessed, &item.IsProtected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan paperdoll item: %w", err)
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

// GetWarehouse retrieves items in warehouse locations
func (r *ItemRepositoryImpl) GetWarehouse(ctx context.Context, charID int32, location models.ItemLocation) ([]models.CharacterItem, error) {
	query := `
		SELECT object_id, owner_id, item_id, count, loc, loc_data, enchant_level, created_at,
			   custom_type1, custom_type2, mana_left, time, augmentation_id,
			   augmentation_skill1, augmentation_skill2, attribute_fire, attribute_water,
			   attribute_wind, attribute_earth, attribute_holy, attribute_dark,
			   visual_id, is_blessed, is_protected
		FROM character_items 
		WHERE owner_id = $1 AND loc = $2
		ORDER BY object_id`

	rows, err := r.db.Query(ctx, query, charID, string(location))
	if err != nil {
		return nil, fmt.Errorf("failed to query warehouse items: %w", err)
	}
	defer rows.Close()

	var items []models.CharacterItem
	for rows.Next() {
		var item models.CharacterItem

		err := rows.Scan(
			&item.ObjectID, &item.OwnerID, &item.ItemID, &item.Count,
			&item.Loc, &item.LocData, &item.EnchantLevel, &item.CreatedAt,
			&item.CustomType1, &item.CustomType2, &item.ManaLeft, &item.Time,
			&item.AugmentationID, &item.AugmentationSkill1, &item.AugmentationSkill2,
			&item.AttributeFire, &item.AttributeWater, &item.AttributeWind,
			&item.AttributeEarth, &item.AttributeHoly, &item.AttributeDark,
			&item.VisualID, &item.IsBlessed, &item.IsProtected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan warehouse item: %w", err)
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

// GetByItemID retrieves all items with specified item template ID
func (r *ItemRepositoryImpl) GetByItemID(ctx context.Context, charID int32, itemID int32) ([]models.CharacterItem, error) {
	query := `
		SELECT object_id, owner_id, item_id, count, loc, loc_data, enchant_level, created_at,
			   custom_type1, custom_type2, mana_left, time, augmentation_id,
			   augmentation_skill1, augmentation_skill2, attribute_fire, attribute_water,
			   attribute_wind, attribute_earth, attribute_holy, attribute_dark,
			   visual_id, is_blessed, is_protected
		FROM character_items 
		WHERE owner_id = $1 AND item_id = $2
		ORDER BY object_id`

	rows, err := r.db.Query(ctx, query, charID, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query items by item ID: %w", err)
	}
	defer rows.Close()

	var items []models.CharacterItem
	for rows.Next() {
		var item models.CharacterItem

		err := rows.Scan(
			&item.ObjectID, &item.OwnerID, &item.ItemID, &item.Count,
			&item.Loc, &item.LocData, &item.EnchantLevel, &item.CreatedAt,
			&item.CustomType1, &item.CustomType2, &item.ManaLeft, &item.Time,
			&item.AugmentationID, &item.AugmentationSkill1, &item.AugmentationSkill2,
			&item.AttributeFire, &item.AttributeWater, &item.AttributeWind,
			&item.AttributeEarth, &item.AttributeHoly, &item.AttributeDark,
			&item.VisualID, &item.IsBlessed, &item.IsProtected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item by item ID: %w", err)
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

// GetEquippedItem retrieves item equipped in specific paperdoll slot
func (r *ItemRepositoryImpl) GetEquippedItem(ctx context.Context, charID int32, slot models.PaperdollSlot) (*models.CharacterItem, error) {
	query := `
		SELECT object_id, owner_id, item_id, count, loc, loc_data, enchant_level, created_at,
			   custom_type1, custom_type2, mana_left, time, augmentation_id,
			   augmentation_skill1, augmentation_skill2, attribute_fire, attribute_water,
			   attribute_wind, attribute_earth, attribute_holy, attribute_dark,
			   visual_id, is_blessed, is_protected
		FROM character_items 
		WHERE owner_id = $1 AND loc = 'PAPERDOLL' AND loc_data = $2`

	var item models.CharacterItem

	err := r.db.QueryRow(ctx, query, charID, int(slot)).Scan(
		&item.ObjectID, &item.OwnerID, &item.ItemID, &item.Count,
		&item.Loc, &item.LocData, &item.EnchantLevel, &item.CreatedAt,
		&item.CustomType1, &item.CustomType2, &item.ManaLeft, &item.Time,
		&item.AugmentationID, &item.AugmentationSkill1, &item.AugmentationSkill2,
		&item.AttributeFire, &item.AttributeWater, &item.AttributeWind,
		&item.AttributeEarth, &item.AttributeHoly, &item.AttributeDark,
		&item.VisualID, &item.IsBlessed, &item.IsProtected,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get equipped item: %w", err)
	}

	return &item, nil
}

// UnequipSlot moves item from paperdoll slot to inventory
func (r *ItemRepositoryImpl) UnequipSlot(ctx context.Context, charID int32, slot models.PaperdollSlot) error {
	_, err := r.db.Exec(ctx,
		"UPDATE character_items SET loc = 'INVENTORY', loc_data = -1 WHERE owner_id = $1 AND loc = 'PAPERDOLL' AND loc_data = $2",
		charID, int(slot))
	if err != nil {
		return fmt.Errorf("failed to unequip slot: %w", err)
	}
	return nil
}

// EquipItem moves item to paperdoll slot
func (r *ItemRepositoryImpl) EquipItem(ctx context.Context, objectID int32, slot models.PaperdollSlot) error {
	_, err := r.db.Exec(ctx,
		"UPDATE character_items SET loc = 'PAPERDOLL', loc_data = $2 WHERE object_id = $1",
		objectID, int(slot))
	if err != nil {
		return fmt.Errorf("failed to equip item: %w", err)
	}
	return nil
}

// GetInventoryWeight calculates total inventory weight (placeholder - needs item templates)
func (r *ItemRepositoryImpl) GetInventoryWeight(ctx context.Context, charID int32) (int, error) {
	// This would require item template data to calculate accurate weight
	// For now, return item count as basic weight estimation
	var weight int
	err := r.db.QueryRow(ctx,
		"SELECT COALESCE(SUM(count), 0) FROM character_items WHERE owner_id = $1 AND loc = 'INVENTORY'",
		charID).Scan(&weight)
	if err != nil {
		return 0, fmt.Errorf("failed to get inventory weight: %w", err)
	}
	return weight, nil
}

// GetItemCount returns total count of specific item type
func (r *ItemRepositoryImpl) GetItemCount(ctx context.Context, charID int32, itemID int32) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx,
		"SELECT COALESCE(SUM(count), 0) FROM character_items WHERE owner_id = $1 AND item_id = $2",
		charID, itemID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get item count: %w", err)
	}
	return count, nil
}

// FindStackableItem finds existing stackable item to add count to
func (r *ItemRepositoryImpl) FindStackableItem(ctx context.Context, charID int32, itemID int32, location models.ItemLocation) (*models.CharacterItem, error) {
	query := `
		SELECT object_id, owner_id, item_id, count, loc, loc_data, enchant_level, created_at,
			   custom_type1, custom_type2, mana_left, time, augmentation_id,
			   augmentation_skill1, augmentation_skill2, attribute_fire, attribute_water,
			   attribute_wind, attribute_earth, attribute_holy, attribute_dark,
			   visual_id, is_blessed, is_protected
		FROM character_items 
		WHERE owner_id = $1 AND item_id = $2 AND loc = $3 AND enchant_level = 0 AND augmentation_id = 0
		ORDER BY count DESC
		LIMIT 1`

	var item models.CharacterItem

	err := r.db.QueryRow(ctx, query, charID, itemID, string(location)).Scan(
		&item.ObjectID, &item.OwnerID, &item.ItemID, &item.Count,
		&item.Loc, &item.LocData, &item.EnchantLevel, &item.CreatedAt,
		&item.CustomType1, &item.CustomType2, &item.ManaLeft, &item.Time,
		&item.AugmentationID, &item.AugmentationSkill1, &item.AugmentationSkill2,
		&item.AttributeFire, &item.AttributeWater, &item.AttributeWind,
		&item.AttributeEarth, &item.AttributeHoly, &item.AttributeDark,
		&item.VisualID, &item.IsBlessed, &item.IsProtected,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find stackable item: %w", err)
	}

	return &item, nil
}