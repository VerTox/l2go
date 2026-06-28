package repo

import (
	"context"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// CharacterRepository defines the interface for character data access
type CharacterRepository interface {
	// Character CRUD operations
	GetByAccount(ctx context.Context, accountName string) ([]models.Character, error)
	GetByID(ctx context.Context, charID int32) (*models.Character, error)
	GetByName(ctx context.Context, name string) (*models.Character, error)
	GetByAccountSlotID(ctx context.Context, accountName string, charSlot int32) (*models.Character, error)
	Create(ctx context.Context, char *models.Character) error
	Update(ctx context.Context, char *models.Character) error
	Delete(ctx context.Context, charID int32) error

	// Character queries
	GetCount(ctx context.Context, accountName string) (int, int, error) // total, inDeletion
	IsNameTaken(ctx context.Context, name string) (bool, error)
	GetBySlot(ctx context.Context, accountName string, slot int) (*models.Character, error)
	GetMaxSlot(ctx context.Context, accountName string) (int, error)
	GetDeleteCandidates(ctx context.Context) ([]models.Character, error) // characters ready for deletion

	// Character state management
	SetOnlineStatus(ctx context.Context, charID int32, online bool) error
	UpdatePosition(ctx context.Context, charID int32, x, y, z int, heading int) error
	UpdateLastAccess(ctx context.Context, charID int32) error

	// Character stats
	UpdateStats(ctx context.Context, charID int32, hp, mp, cp float64) error
	UpdateExperience(ctx context.Context, charID int32, exp int64, sp int) error
	UpdateKarma(ctx context.Context, charID int32, karma int) error
}

// ItemRepository defines the interface for character items data access
type ItemRepository interface {
	// Item CRUD operations
	GetByCharacter(ctx context.Context, charID int32) ([]models.CharacterItem, error)
	GetByObjectID(ctx context.Context, objectID int32) (*models.CharacterItem, error)
	Create(ctx context.Context, item *models.CharacterItem) error
	Update(ctx context.Context, item *models.CharacterItem) error
	Delete(ctx context.Context, objectID int32) error
	DeleteByCharacter(ctx context.Context, charID int32) error // cleanup when character deleted

	// Item location queries
	GetInventory(ctx context.Context, charID int32) ([]models.CharacterItem, error)
	GetPaperdoll(ctx context.Context, charID int32) ([]models.CharacterItem, error)
	GetWarehouse(ctx context.Context, charID int32, location models.ItemLocation) ([]models.CharacterItem, error)
	GetByItemID(ctx context.Context, charID int32, itemID int32) ([]models.CharacterItem, error)

	// Equipment operations
	GetEquippedItem(ctx context.Context, charID int32, slot models.PaperdollSlot) (*models.CharacterItem, error)
	UnequipSlot(ctx context.Context, charID int32, slot models.PaperdollSlot) error
	EquipItem(ctx context.Context, objectID int32, slot models.PaperdollSlot) error

	// Inventory management
	GetInventoryWeight(ctx context.Context, charID int32) (int, error)
	GetItemCount(ctx context.Context, charID int32, itemID int32) (int64, error)
	FindStackableItem(ctx context.Context, charID int32, itemID int32, location models.ItemLocation) (*models.CharacterItem, error)
}

// SkillRepository defines the interface for character skills data access
type SkillRepository interface {
	// Skill CRUD operations
	GetByCharacter(ctx context.Context, charID int32) ([]models.CharacterSkill, error)
	GetSkill(ctx context.Context, charID int32, skillID int32) (*models.CharacterSkill, error)
	LearnSkill(ctx context.Context, charID int32, skillID int32, level int) error
	UpdateSkill(ctx context.Context, charID int32, skillID int32, level int) error
	ForgetSkill(ctx context.Context, charID int32, skillID int32) error
	DeleteByCharacter(ctx context.Context, charID int32) error // cleanup when character deleted

	// Skill queries
	HasSkill(ctx context.Context, charID int32, skillID int32) (bool, error)
	GetSkillLevel(ctx context.Context, charID int32, skillID int32) (int, error)
	GetSkillsByType(ctx context.Context, charID int32, skillType int) ([]models.CharacterSkill, error)

	// Skill effects management
	AddSkillEffect(ctx context.Context, charID int32, skillID int32, remainingTime int) error
	RemoveSkillEffect(ctx context.Context, charID int32, skillID int32) error
	GetActiveEffects(ctx context.Context, charID int32) ([]models.CharacterSkillEffect, error)
	CleanupExpiredEffects(ctx context.Context) error
}

// ShortcutRepository defines the interface for character shortcuts data access
type ShortcutRepository interface {
	// Shortcut CRUD operations
	GetByCharacter(ctx context.Context, charID int32) ([]models.CharacterShortcut, error)
	GetByPage(ctx context.Context, charID int32, page int) ([]models.CharacterShortcut, error)
	GetBySlot(ctx context.Context, charID int32, slot, page int) (*models.CharacterShortcut, error)
	SetShortcut(ctx context.Context, shortcut *models.CharacterShortcut) error
	DeleteShortcut(ctx context.Context, charID int32, slot, page int) error
	DeleteByCharacter(ctx context.Context, charID int32) error // cleanup when character deleted

	// Shortcut operations
	ClearPage(ctx context.Context, charID int32, page int) error
	GetMaxPage(ctx context.Context, charID int32) (int, error)
}

// SpawnRepository defines the interface for NPC spawnlist data access
type SpawnRepository interface {
	// GetAll returns all spawn entries from the database
	GetAll(ctx context.Context) ([]models.SpawnData, error)
	// GetCount returns the total number of spawn entries
	GetCount(ctx context.Context) (int, error)
	// BulkInsert inserts a batch of spawn entries
	BulkInsert(ctx context.Context, spawns []models.SpawnData) (int, error)
}

// Repository aggregates all repository interfaces for dependency injection
type Repository struct {
	Character CharacterRepository
	Item      ItemRepository
	Skill     SkillRepository
	Shortcut  ShortcutRepository
	Spawn     SpawnRepository
}

// Transaction defines transaction interface for atomic operations
type Transaction interface {
	// Transaction control
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error

	// Repository access within transaction
	Character() CharacterRepository
	Item() ItemRepository
	Skill() SkillRepository
	Shortcut() ShortcutRepository
}

// TransactionManager defines interface for transaction management
type TransactionManager interface {
	// Begin a new transaction
	BeginTransaction(ctx context.Context) (Transaction, error)

	// Execute function within transaction with automatic commit/rollback
	WithTransaction(ctx context.Context, fn func(tx Transaction) error) error
}

// HealthChecker defines interface for repository health checking
type HealthChecker interface {
	// Check if repository is healthy
	HealthCheck(ctx context.Context) error
}

// DatabaseRepository combines all database operations
type DatabaseRepository interface {
	TransactionManager
	HealthChecker

	// Repository access
	Character() CharacterRepository
	Item() ItemRepository
	Skill() SkillRepository
	Shortcut() ShortcutRepository
	Spawn() SpawnRepository
}
