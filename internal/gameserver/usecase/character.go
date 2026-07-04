package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// CharacterUseCase handles character business logic
type CharacterUseCase struct {
	repo repo.DatabaseRepository
}

// NewCharacterUseCase creates a new character use case
func NewCharacterUseCase(repo repo.DatabaseRepository) *CharacterUseCase {
	return &CharacterUseCase{
		repo: repo,
	}
}

// GetCharacterList retrieves all characters for an account
func (uc *CharacterUseCase) GetCharacterList(ctx context.Context, accountName string) ([]models.Character, error) {
	characters, err := uc.repo.Character().GetByAccount(ctx, accountName)
	if err != nil {
		return nil, fmt.Errorf("failed to get character list: %w", err)
	}

	return characters, nil
}

// GetCharacterListEntries retrieves characters with additional display information
func (uc *CharacterUseCase) GetCharacterListEntries(ctx context.Context, accountName string) ([]models.CharacterListEntry, error) {
	characters, err := uc.GetCharacterList(ctx, accountName)
	if err != nil {
		return nil, err
	}

	entries := make([]models.CharacterListEntry, len(characters))
	for i, char := range characters {
		entries[i] = char.ToListEntry()

		// Load paperdoll items for display
		paperdollItems, err := uc.repo.Item().GetPaperdoll(ctx, char.ID)
		if err != nil {
			// Log error but don't fail the request
			// In production, use proper logging
			continue
		}
		entries[i].PaperdollItems = paperdollItems
	}

	return entries, nil
}

// CreateCharacter creates a new character with validation
func (uc *CharacterUseCase) CreateCharacter(ctx context.Context, req *models.CharacterCreateRequest) (*models.Character, error) {
	// Canonicalize the account name so characters are always stored lowercase,
	// regardless of the case the request arrived in. (l2go-xhp)
	req.AccountName = models.NormalizeAccountName(req.AccountName)

	// Validate request
	if err := uc.validateCharacterCreation(ctx, req); err != nil {
		return nil, err
	}

	// Get starting template for race/class
	template, err := uc.getCharacterTemplate(req.Race, req.ClassID)
	if err != nil {
		return nil, fmt.Errorf("invalid race/class combination: %w", err)
	}

	// Find available slot
	slot, err := uc.findAvailableSlot(ctx, req.AccountName)
	if err != nil {
		return nil, err
	}

	// Create character using transaction to ensure consistency
	var newChar *models.Character
	err = uc.repo.WithTransaction(ctx, func(tx repo.Transaction) error {
		// Create character
		char := uc.buildNewCharacter(req, template, slot)
		if err := tx.Character().Create(ctx, char); err != nil {
			return fmt.Errorf("failed to create character: %w", err)
		}

		// Create starting items
		if err := uc.createStartingItems(ctx, tx, char.ID, template); err != nil {
			return fmt.Errorf("failed to create starting items: %w", err)
		}

		// Grant auto-get skills for the starting class/level (Lucky, Common Craft, ...)
		if _, err := uc.grantAutoGetSkills(ctx, tx.Skill(), char, nil); err != nil {
			return fmt.Errorf("failed to grant starting skills: %w", err)
		}

		newChar = char
		return nil
	})

	if err != nil {
		return nil, err
	}

	return newChar, nil
}

// SelectCharacter validates and loads a character for play
func (uc *CharacterUseCase) SelectCharacter(ctx context.Context, charID int32, accountName string) (*models.Character, error) {
	// Load character
	char, err := uc.repo.Character().GetByAccountSlotID(ctx, accountName, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to load character: %w", err)
	}

	if char == nil {
		return nil, models.ErrCharacterNotFound
	}

	// Validate ownership
	if char.AccountName != accountName {
		return nil, fmt.Errorf("character does not belong to account")
	}

	// Check if character is marked for deletion
	if char.IsMarkedForDeletion() && !char.CanBeDeleted() {
		return nil, fmt.Errorf("character is marked for deletion")
	}

	// Update last access time
	if err := uc.repo.Character().UpdateLastAccess(ctx, charID); err != nil {
		// Log error but don't fail selection
		// In production, use proper logging
	}

	items, err := uc.repo.Item().GetPaperdoll(ctx, char.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load character items: %w", err)
	}

	for _, item := range items {
		if item.LocData >= 0 && item.LocData < len(char.PaperdollItems) {
			char.PaperdollItems[item.LocData] = item.ItemID
			char.PaperdollObjectIDs[item.LocData] = item.ObjectID
		}
	}

	return char, nil
}

// DeleteCharacter marks a character for deletion or deletes immediately
func (uc *CharacterUseCase) DeleteCharacter(ctx context.Context, charID int32, accountName string) error {
	// Load character
	char, err := uc.repo.Character().GetByID(ctx, charID)
	if err != nil {
		return fmt.Errorf("failed to load character: %w", err)
	}

	if char == nil {
		return models.ErrCharacterNotFound
	}

	// Validate ownership
	if char.AccountName != accountName {
		return fmt.Errorf("character does not belong to account")
	}

	// Check if character can be deleted immediately
	if char.CanBeDeleted() {
		return uc.permanentlyDeleteCharacter(ctx, charID)
	}

	// Mark for deletion with 7-day timer
	char.MarkForDeletion(7 * 24 * time.Hour)
	if err := uc.repo.Character().Update(ctx, char); err != nil {
		return fmt.Errorf("failed to mark character for deletion: %w", err)
	}

	return nil
}

// CancelCharacterDeletion cancels character deletion
func (uc *CharacterUseCase) CancelCharacterDeletion(ctx context.Context, charID int32, accountName string) error {
	// Load character
	char, err := uc.repo.Character().GetByID(ctx, charID)
	if err != nil {
		return fmt.Errorf("failed to load character: %w", err)
	}

	if char == nil {
		return models.ErrCharacterNotFound
	}

	// Validate ownership
	if char.AccountName != accountName {
		return fmt.Errorf("character does not belong to account")
	}

	// Check if marked for deletion
	if !char.IsMarkedForDeletion() {
		return fmt.Errorf("character is not marked for deletion")
	}

	// Cancel deletion
	char.CancelDeletion()
	if err := uc.repo.Character().Update(ctx, char); err != nil {
		return fmt.Errorf("failed to cancel character deletion: %w", err)
	}

	return nil
}

func (uc *CharacterUseCase) GetCharacterInventory(ctx context.Context, charID int32) ([]models.CharacterItem, error) {
	items, err := uc.repo.Item().GetInventory(ctx, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to load character inventory: %w", err)
	}

	return items, nil
}

// GetCharacterTemplates returns available character templates
func (uc *CharacterUseCase) GetCharacterTemplates(ctx context.Context) ([]CharacterTemplate, error) {
	// Return predefined L2J character templates
	return getDefaultCharacterTemplates(), nil
}

// ProcessScheduledDeletions permanently deletes characters whose deletion timer has expired
func (uc *CharacterUseCase) ProcessScheduledDeletions(ctx context.Context) (int, error) {
	candidates, err := uc.repo.Character().GetDeleteCandidates(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get delete candidates: %w", err)
	}

	deletedCount := 0
	for _, char := range candidates {
		if err := uc.permanentlyDeleteCharacter(ctx, char.ID); err != nil {
			// Log error but continue with other characters
			// In production, use proper logging
			continue
		}
		deletedCount++
	}

	return deletedCount, nil
}

// validateCharacterCreation validates character creation request
func (uc *CharacterUseCase) validateCharacterCreation(ctx context.Context, req *models.CharacterCreateRequest) error {
	// Validate name
	if err := uc.validateCharacterName(req.Name); err != nil {
		return err
	}

	// Check if name is taken
	taken, err := uc.repo.Character().IsNameTaken(ctx, req.Name)
	if err != nil {
		return fmt.Errorf("failed to check name availability: %w", err)
	}
	if taken {
		return models.ErrCharacterExists
	}

	// Validate race
	if !isValidRace(req.Race) {
		return models.ErrInvalidRace
	}

	// Validate sex
	if !isValidSex(req.Sex) {
		return models.ErrInvalidSex
	}

	// Validate class for race
	if !isValidClassForRace(req.Race, req.ClassID) {
		return models.ErrInvalidClass
	}

	// Check character count limit
	total, _, err := uc.repo.Character().GetCount(ctx, req.AccountName)
	if err != nil {
		return fmt.Errorf("failed to check character count: %w", err)
	}

	if total >= 7 { // L2J default character limit
		return models.ErrTooManyCharacters
	}

	return nil
}

// validateCharacterName validates character name according to L2J rules
func (uc *CharacterUseCase) validateCharacterName(name string) error {
	// Length check
	if len(name) < 1 || len(name) > 16 {
		return models.ErrInvalidCharacterName
	}

	// Character validation (only letters)
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return models.ErrInvalidCharacterName
		}
	}

	// Reserved names check
	lowerName := strings.ToLower(name)
	reservedNames := []string{"admin", "gm", "mod", "moderator", "support"}
	for _, reserved := range reservedNames {
		if lowerName == reserved {
			return models.ErrInvalidCharacterName
		}
	}

	return nil
}

// findAvailableSlot finds the next available character slot
func (uc *CharacterUseCase) findAvailableSlot(ctx context.Context, accountName string) (int, error) {
	chars, err := uc.repo.Character().GetByAccount(ctx, accountName)
	if err != nil {
		return 0, err
	}

	// Find first available slot (0-6)
	used := make(map[int]bool)
	for _, char := range chars {
		used[char.CharSlot] = true
	}

	for slot := 0; slot < 7; slot++ {
		if !used[slot] {
			return slot, nil
		}
	}

	return 0, fmt.Errorf("no available character slots")
}

// buildNewCharacter constructs a new character from request and template
func (uc *CharacterUseCase) buildNewCharacter(req *models.CharacterCreateRequest, template *CharacterTemplate, slot int) *models.Character {
	now := time.Now()

	// Compute HP/MP/CP with stat bonuses applied
	maxHP := models.ComputeMaxHP(template.BaseStats.HP, template.BaseStats.CON)
	maxMP := models.ComputeMaxMP(template.BaseStats.MP, template.BaseStats.MEN)
	maxCP := models.ComputeMaxCP(template.BaseStats.CP, template.BaseStats.CON)

	return &models.Character{
		AccountName:       req.AccountName,
		Name:              req.Name,
		Level:             1,
		MaxHP:             maxHP,
		CurrentHP:         float64(maxHP),
		MaxMP:             maxMP,
		CurrentMP:         float64(maxMP),
		MaxCP:             maxCP,
		CurrentCP:         maxCP,
		Face:              req.Face,
		HairStyle:         req.HairStyle,
		HairColor:         req.HairColor,
		Sex:               req.Sex,
		Experience:        0,
		SP:                0,
		Karma:             0,
		PKKills:           0,
		PvPKills:          0,
		ClanID:            0,
		Race:              req.Race,
		ClassID:           req.ClassID,
		BaseClass:         req.ClassID,
		DeleteTime:        0,
		VitalityPoints:    2000,
		AccessLevel:       0,
		Position:          template.StartingPosition,
		Heading:           0,
		CreatedAt:         now,
		LastAccess:        now.Unix(),
		OnlineTime:        0,
		OnlineStatus:      false,
		CharSlot:          slot,
		Newbie:            true,
		Noble:             false,
		Hero:              false,
		HeroEndDate:       nil,
		DeathPenaltyLevel: 0,
		Title:             "",
		RecHave:           0,
		RecLeft:           10,
		Fame:              0,
		FishingPoints:     0,
		// Base stats from class template
		BaseSTR: template.BaseStats.STR,
		BaseDEX: template.BaseStats.DEX,
		BaseCON: template.BaseStats.CON,
		BaseINT: template.BaseStats.INT,
		BaseWIT: template.BaseStats.WIT,
		BaseMEN: template.BaseStats.MEN,
	}
}

// createStartingItems creates initial equipment for new character
func (uc *CharacterUseCase) createStartingItems(ctx context.Context, tx repo.Transaction, charID int32, template *CharacterTemplate) error {
	for _, itemData := range template.StartingItems {
		item := &models.CharacterItem{
			OwnerID:      charID,
			ItemID:       itemData.ItemID,
			Count:        itemData.Count,
			Loc:          string(itemData.Location),
			LocData:      itemData.LocData,
			EnchantLevel: 0,
		}

		if err := tx.Item().Create(ctx, item); err != nil {
			return fmt.Errorf("failed to create starting item %d: %w", itemData.ItemID, err)
		}
	}

	return nil
}

// grantAutoGetSkills grants the class skill tree's auto-get skills for the
// character's current class and level, skipping any already known at >= that
// level. Works against either a transaction's or the pool's skill repository, so
// it serves both creation and world-entry reconciliation. Returns the skills
// actually written. Mirrors L2J SkillTreesData.getAvailableAutoGetSkills /
// giveAvailableSkills, restricted to the auto-get subset (learn-by-NPC skills
// with SP costs are a later phase, l2go-hv9).
func (uc *CharacterUseCase) grantAutoGetSkills(ctx context.Context, skillRepo repo.SkillRepository, char *models.Character, existing []models.CharacterSkill) ([]registry.AutoGetSkill, error) {
	desired := registry.GetSkillTreeRegistry().AutoGetSkills(char.ClassID, char.Level)
	if len(desired) == 0 {
		return nil, nil
	}
	have := make(map[int32]int, len(existing))
	for _, s := range existing {
		have[s.SkillID] = s.SkillLevel
	}

	var granted []registry.AutoGetSkill
	for _, d := range desired {
		if lvl, ok := have[d.SkillID]; ok && lvl >= d.Level {
			continue // already known at this level or higher
		}
		if err := skillRepo.LearnSkill(ctx, char.ID, d.SkillID, d.Level); err != nil {
			return granted, fmt.Errorf("failed to grant auto-get skill %d: %w", d.SkillID, err)
		}
		granted = append(granted, d)
	}
	return granted, nil
}

// ReconcileAutoGetSkills grants any auto-get skills the character should have at
// its current class/level but is missing (new characters created before this
// system, or level-ups that happened while offline). Idempotent: a fully
// up-to-date character gets no writes. Returns the skills newly granted.
func (uc *CharacterUseCase) ReconcileAutoGetSkills(ctx context.Context, char *models.Character) ([]registry.AutoGetSkill, error) {
	existing, err := uc.repo.Skill().GetByCharacter(ctx, char.ID)
	if err != nil {
		return nil, err
	}
	return uc.grantAutoGetSkills(ctx, uc.repo.Skill(), char, existing)
}

// permanentlyDeleteCharacter completely removes character and all associated data
func (uc *CharacterUseCase) permanentlyDeleteCharacter(ctx context.Context, charID int32) error {
	return uc.repo.WithTransaction(ctx, func(tx repo.Transaction) error {
		// Delete all character data
		if err := tx.Item().DeleteByCharacter(ctx, charID); err != nil {
			return fmt.Errorf("failed to delete character items: %w", err)
		}

		if err := tx.Skill().DeleteByCharacter(ctx, charID); err != nil {
			return fmt.Errorf("failed to delete character skills: %w", err)
		}

		if err := tx.Shortcut().DeleteByCharacter(ctx, charID); err != nil {
			return fmt.Errorf("failed to delete character shortcuts: %w", err)
		}

		if err := tx.Character().Delete(ctx, charID); err != nil {
			return fmt.Errorf("failed to delete character: %w", err)
		}

		return nil
	})
}

// GetCharacterAllItems retrieves all items (inventory + paperdoll) for a character
func (uc *CharacterUseCase) GetCharacterAllItems(ctx context.Context, charID int32) ([]models.CharacterItem, error) {
	items, err := uc.repo.Item().GetByCharacter(ctx, charID)
	if err != nil {
		return nil, fmt.Errorf("failed to get character items: %w", err)
	}
	return items, nil
}

// GetCharacterSkills loads all learned skills for a character (world entry →
// SkillList). Returns the raw character_skills rows; the caller resolves each
// against SkillData for the passive/enchanted flags.
func (uc *CharacterUseCase) GetCharacterSkills(ctx context.Context, charID int32) ([]models.CharacterSkill, error) {
	return uc.repo.Skill().GetByCharacter(ctx, charID)
}

// GetShortcuts loads all persisted quick-bar shortcuts for a character (world entry).
func (uc *CharacterUseCase) GetShortcuts(ctx context.Context, charID int32) ([]models.CharacterShortcut, error) {
	return uc.repo.Shortcut().GetByCharacter(ctx, charID)
}

// SaveShortcut upserts a single shortcut (RequestShortCutReg).
func (uc *CharacterUseCase) SaveShortcut(ctx context.Context, sc *models.CharacterShortcut) error {
	return uc.repo.Shortcut().SetShortcut(ctx, sc)
}

// DeleteShortcut removes the shortcut at slot/page (RequestShortCutDel).
func (uc *CharacterUseCase) DeleteShortcut(ctx context.Context, charID int32, slot, page int) error {
	return uc.repo.Shortcut().DeleteShortcut(ctx, charID, slot, page)
}
