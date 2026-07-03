package models

import (
	"time"
)

// Character represents a player character in the game world
type Character struct {
	// Primary identification
	ID          int32  `json:"char_id" db:"char_id"`
	AccountName string `json:"account_name" db:"account_name"`
	Name        string `json:"char_name" db:"char_name"`

	// Core character attributes
	Level int `json:"level" db:"level"`

	// Health and mana
	MaxHP     int     `json:"max_hp" db:"max_hp"`
	CurrentHP float64 `json:"cur_hp" db:"cur_hp"`
	MaxMP     int     `json:"max_mp" db:"max_mp"`
	CurrentMP float64 `json:"cur_mp" db:"cur_mp"`
	MaxCP     int     `json:"max_cp" db:"max_cp"`
	CurrentCP int     `json:"cur_cp" db:"cur_cp"`

	// Character appearance
	Face      int `json:"face" db:"face"`
	HairStyle int `json:"hair_style" db:"hair_style"`
	HairColor int `json:"hair_color" db:"hair_color"`
	Sex       int `json:"sex" db:"sex"`

	// Experience and skill points
	Experience int64 `json:"exp" db:"exp"`
	SP         int   `json:"sp" db:"sp"`

	// PvP and karma system
	Karma    int `json:"karma" db:"karma"`
	PKKills  int `json:"pk_kills" db:"pk_kills"`
	PvPKills int `json:"pvp_kills" db:"pvp_kills"`

	// Clan and social
	ClanID int `json:"clan_id" db:"clan_id"`

	// Character class system
	Race      int `json:"race" db:"race"`
	ClassID   int `json:"class_id" db:"class_id"`
	BaseClass int `json:"base_class" db:"base_class"`

	// Deletion system
	DeleteTime int64 `json:"delete_time" db:"delete_time"`

	// Game systems
	VitalityPoints int `json:"vitality_points" db:"vitality_points"`
	AccessLevel    int `json:"access_level" db:"access_level"`

	// World position
	Position Position `json:"position"`
	Heading  int      `json:"heading" db:"heading"`

	// Metadata
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	LastAccess int64     `json:"last_access" db:"last_access"`

	// Additional L2J fields
	OnlineTime   int  `json:"online_time" db:"online_time"`
	OnlineStatus bool `json:"online_status" db:"online_status"`
	CharSlot     int  `json:"char_slot" db:"char_slot"`
	Newbie       bool `json:"newbie" db:"newbie"`
	Noble        bool `json:"noble" db:"noble"`
	Hero         bool `json:"hero" db:"hero"`

	// Hero system
	HeroEndDate *time.Time `json:"hero_end_date" db:"hero_end_date"`

	// Death penalty
	DeathPenaltyLevel int `json:"death_penalty_level" db:"death_penalty_level"`

	// Title and recommendation
	Title   string `json:"title" db:"title"`
	RecHave int    `json:"rec_have" db:"rec_have"`
	RecLeft int    `json:"rec_left" db:"rec_left"`

	// Fame and fishing
	Fame           int       `json:"fame" db:"fame"`
	FishingPoints  int       `json:"fishing_points" db:"fishing_points"`
	PaperdollItems [26]int32 `json:"paperdoll_items" db:"paperdoll_items"` // Equipped item template (display) IDs per paperdoll slot
	// PaperdollObjectIDs — instance (object) IDs per paperdoll slot, parallel to
	// PaperdollItems. Нужны для UserInfo (флаг оружия + корректная отрисовка), чтобы
	// game loop мог собрать папердолл без обращения к БД.
	PaperdollObjectIDs [26]int32 `json:"paperdoll_object_ids"`

	// Base stats (STR/DEX/CON/INT/WIT/MEN) — set from class template at creation
	BaseSTR int `json:"base_str" db:"base_str"`
	BaseDEX int `json:"base_dex" db:"base_dex"`
	BaseCON int `json:"base_con" db:"base_con"`
	BaseINT int `json:"base_int" db:"base_int"`
	BaseWIT int `json:"base_wit" db:"base_wit"`
	BaseMEN int `json:"base_men" db:"base_men"`

	// StatMods are the active stat modifiers layered on top of ComputeStats —
	// populated at world entry from passive skills (epic l2go-z36, l2go-9ep) and,
	// later, timed buffs. Runtime-only, never persisted. Mutation follows the same
	// discipline as other Character progress: the game loop is the sole writer once
	// the player is live; packet builders read snapshots.
	StatMods []StatModifier `json:"-" db:"-"`
}

// Position represents a character's location in the world
type Position struct {
	X int `json:"x" db:"x"`
	Y int `json:"y" db:"y"`
	Z int `json:"z" db:"z"`
}

// CharacterStats represents character's base statistics
type CharacterStats struct {
	STR int `json:"str"`
	DEX int `json:"dex"`
	CON int `json:"con"`
	INT int `json:"int"`
	WIT int `json:"wit"`
	MEN int `json:"men"`
}

// CharacterRace represents character race constants
type CharacterRace int

const (
	RaceHuman   CharacterRace = 0
	RaceElf     CharacterRace = 1
	RaceDarkElf CharacterRace = 2
	RaceOrc     CharacterRace = 3
	RaceDwarf   CharacterRace = 4
	RaceKamael  CharacterRace = 5
)

// CharacterSex represents character gender constants
type CharacterSex int

const (
	SexMale   CharacterSex = 0
	SexFemale CharacterSex = 1
)

// CharacterClass represents base character classes
type CharacterClass int

const (
	// Human classes
	ClassHumanFighter CharacterClass = 0
	ClassHumanMystic  CharacterClass = 10

	// Elf classes
	ClassElfFighter CharacterClass = 18
	ClassElfMystic  CharacterClass = 25

	// Dark Elf classes
	ClassDarkElfFighter CharacterClass = 31
	ClassDarkElfMystic  CharacterClass = 38

	// Orc classes
	ClassOrcFighter CharacterClass = 44
	ClassOrcMystic  CharacterClass = 49

	// Dwarf classes
	ClassDwarfFighter CharacterClass = 53

	// Kamael classes
	ClassKamaelSoldier CharacterClass = 123
)

// IsAlive returns true if character has HP > 0
func (c *Character) IsAlive() bool {
	return c.CurrentHP > 0
}

// IsOnline returns true if character is currently online
func (c *Character) IsOnline() bool {
	return c.OnlineStatus
}

// IsMarkedForDeletion returns true if character is scheduled for deletion
func (c *Character) IsMarkedForDeletion() bool {
	return c.DeleteTime > 0
}

// GetDeletionTimeRemaining returns remaining time until deletion (in seconds)
func (c *Character) GetDeletionTimeRemaining() int64 {
	if !c.IsMarkedForDeletion() {
		return 0
	}
	now := time.Now().Unix()
	remaining := c.DeleteTime - now
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CanBeDeleted returns true if character can be deleted immediately
func (c *Character) CanBeDeleted() bool {
	// Characters level 20+ require deletion timer
	if c.Level >= 20 {
		return c.IsMarkedForDeletion() && c.GetDeletionTimeRemaining() == 0
	}
	// Low level characters can be deleted immediately
	return true
}

// IsHero returns true if character is currently a hero
func (c *Character) IsHero() bool {
	if !c.Hero {
		return false
	}
	if c.HeroEndDate == nil {
		return true
	}
	return time.Now().Before(*c.HeroEndDate)
}

// IsNoble returns true if character is a noblesse
func (c *Character) IsNoble() bool {
	return c.Noble
}

// IsNewbie returns true if character is still in newbie status
func (c *Character) IsNewbie() bool {
	return c.Newbie
}

// GetRaceName returns the character's race name
func (c *Character) GetRaceName() string {
	switch CharacterRace(c.Race) {
	case RaceHuman:
		return "Human"
	case RaceElf:
		return "Elf"
	case RaceDarkElf:
		return "Dark Elf"
	case RaceOrc:
		return "Orc"
	case RaceDwarf:
		return "Dwarf"
	case RaceKamael:
		return "Kamael"
	default:
		return "Unknown"
	}
}

// GetSexName returns the character's gender name
func (c *Character) GetSexName() string {
	switch CharacterSex(c.Sex) {
	case SexMale:
		return "Male"
	case SexFemale:
		return "Female"
	default:
		return "Unknown"
	}
}

// HasClan returns true if character belongs to a clan
func (c *Character) HasClan() bool {
	return c.ClanID > 0
}

// GetDistanceTo calculates distance to another position
func (c *Character) GetDistanceTo(pos Position) float64 {
	dx := float64(c.Position.X - pos.X)
	dy := float64(c.Position.Y - pos.Y)
	dz := float64(c.Position.Z - pos.Z)
	return dx*dx + dy*dy + dz*dz // Return squared distance for performance
}

// SetPosition updates character position
func (c *Character) SetPosition(x, y, z int) {
	c.Position = Position{X: x, Y: y, Z: z}
}

// SetHeading updates character heading
func (c *Character) SetHeading(heading int) {
	// Normalize heading to 0-65535 range
	if heading < 0 {
		heading = 0
	} else if heading > 65535 {
		heading = 65535
	}
	c.Heading = heading
}

// MarkForDeletion marks character for deletion after specified duration
func (c *Character) MarkForDeletion(duration time.Duration) {
	c.DeleteTime = time.Now().Add(duration).Unix()
}

// CancelDeletion cancels character deletion
func (c *Character) CancelDeletion() {
	c.DeleteTime = 0
}

// ValidateForCreation validates character data for creation
func (c *Character) ValidateForCreation() error {
	// Name validation
	if len(c.Name) < 1 || len(c.Name) > 16 {
		return ErrInvalidCharacterName
	}

	// Race validation
	if c.Race < 0 || c.Race > 5 {
		return ErrInvalidRace
	}

	// Sex validation
	if c.Sex < 0 || c.Sex > 1 {
		return ErrInvalidSex
	}

	// Class validation (basic validation, more complex validation in use cases)
	if c.ClassID < 0 {
		return ErrInvalidClass
	}

	return nil
}

// Character validation errors
var (
	ErrInvalidCharacterName = &CharacterError{"invalid character name"}
	ErrInvalidRace          = &CharacterError{"invalid race"}
	ErrInvalidSex           = &CharacterError{"invalid sex"}
	ErrInvalidClass         = &CharacterError{"invalid class"}
	ErrCharacterNotFound    = &CharacterError{"character not found"}
	ErrCharacterExists      = &CharacterError{"character already exists"}
	ErrInvalidSlot          = &CharacterError{"invalid character slot"}
)

// CharacterError represents character-related errors
type CharacterError struct {
	msg string
}

func (e *CharacterError) Error() string {
	return e.msg
}

// CharacterSkill represents a learned skill
type CharacterSkill struct {
	CharID     int32     `json:"char_id" db:"char_id"`
	SkillID    int32     `json:"skill_id" db:"skill_id"`
	SkillLevel int       `json:"skill_level" db:"skill_level"`
	ClassIndex int       `json:"class_index" db:"class_index"` // For dual class system
	LearnedAt  time.Time `json:"learned_at" db:"learned_at"`
}

// CharacterSkillEffect represents an active skill effect
type CharacterSkillEffect struct {
	CharID        int32     `json:"char_id" db:"char_id"`
	SkillID       int32     `json:"skill_id" db:"skill_id"`
	SkillLevel    int       `json:"skill_level" db:"skill_level"`
	RemainingTime int       `json:"remaining_time" db:"remaining_time"` // seconds
	AppliedAt     time.Time `json:"applied_at" db:"applied_at"`
}

// CharacterRecipe represents a recipe registered in a character's recipe book.
// Mirrors L2J's character_recipebook row: RecipeID is the internal recipe-list id
// (recipes.xml <item id="..">), NOT the scroll item id or the crafted item id.
type CharacterRecipe struct {
	CharID       int32     `json:"char_id" db:"char_id"`
	RecipeID     int32     `json:"recipe_id" db:"recipe_id"`
	IsDwarven    bool      `json:"is_dwarven" db:"is_dwarven"` // true = dwarven creation, false = common
	ClassIndex   int       `json:"class_index" db:"class_index"`
	RegisteredAt time.Time `json:"registered_at" db:"registered_at"`
}

// CharacterShortcut represents a UI shortcut/macro
type CharacterShortcut struct {
	CharID     int32 `json:"char_id" db:"char_id"`
	Slot       int   `json:"slot" db:"slot"`
	Page       int   `json:"page" db:"page"`
	Type       int   `json:"type" db:"type"` // 1=item, 2=skill, 3=action
	ShortcutID int   `json:"shortcut_id" db:"shortcut_id"`
	Level      int   `json:"level" db:"level"`
	SubLevel   int   `json:"sub_level" db:"sub_level"`
}

// CharacterCreateRequest represents a character creation request
type CharacterCreateRequest struct {
	AccountName string `json:"account_name"`
	Name        string `json:"name"`
	Race        int    `json:"race"`
	Sex         int    `json:"sex"`
	ClassID     int    `json:"class_id"`
	HairStyle   int    `json:"hair_style"`
	HairColor   int    `json:"hair_color"`
	Face        int    `json:"face"`
	Slot        int    `json:"slot"`

	// Starting stats (optional, will use defaults if not provided)
	StartingStats *CharacterStats `json:"starting_stats,omitempty"`
}

// CharacterListEntry represents a character in character selection screen
type CharacterListEntry struct {
	Character

	// Additional display information
	PaperdollItems []CharacterItem `json:"paperdoll_items,omitempty"`
	CanDelete      bool            `json:"can_delete"`
	DeletionTimer  int64           `json:"deletion_timer_sec"`
}

// ToListEntry converts Character to CharacterListEntry
func (c *Character) ToListEntry() CharacterListEntry {
	return CharacterListEntry{
		Character:     *c,
		CanDelete:     c.CanBeDeleted(),
		DeletionTimer: c.GetDeletionTimeRemaining(),
	}
}
