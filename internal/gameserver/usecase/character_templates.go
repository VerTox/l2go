package usecase

import (
	"fmt"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// CharacterTemplate represents a character creation template
type CharacterTemplate struct {
	Race             int                 `json:"race"`
	ClassID          int                 `json:"class_id"`
	ClassName        string              `json:"class_name"`
	BaseStats        CharacterBaseStats  `json:"base_stats"`
	StartingItems    []StartingItemData  `json:"starting_items"`
	StartingSkills   []StartingSkillData `json:"starting_skills"`
	StartingPosition models.Position     `json:"starting_position"`
}

// CharacterBaseStats represents base character statistics
type CharacterBaseStats struct {
	STR int `json:"str"`
	DEX int `json:"dex"`
	CON int `json:"con"`
	INT int `json:"int"`
	WIT int `json:"wit"`
	MEN int `json:"men"`
	HP  int `json:"hp"`
	MP  int `json:"mp"`
	CP  int `json:"cp"`

	// Combat base stats from pcBaseStats.xml
	Combat models.CombatBaseStats `json:"combat"`
}

// StartingItemData represents an item given to new characters
type StartingItemData struct {
	ItemID   int32               `json:"item_id"`
	Count    int64               `json:"count"`
	Location models.ItemLocation `json:"location"`
	LocData  int                 `json:"loc_data"` // paperdoll slot or -1
}

// StartingSkillData represents a skill learned by new characters
type StartingSkillData struct {
	SkillID int32 `json:"skill_id"`
	Level   int   `json:"level"`
}

// fighterCombatStats returns combat base stats for fighter classes
// Values from pcBaseStats.xml: basePAtk=4, baseMAtk=6, baseCritRate=4,
// basePAtkSpd=300, baseMAtkSpd=333
// Fighter PDef: chest=31, legs=18, head=12, feet=7, gloves=8, underwear=3, cloak=1
// All MDef: rear=9, lear=9, rfinger=5, lfinger=5, neck=13
func fighterCombatStats() models.CombatBaseStats {
	return models.CombatBaseStats{
		BasePAtk:    4,
		BaseMAtk:    6,
		BaseCritRate: 4,
		BasePAtkSpd: 300,
		BaseMAtkSpd: 333,

		BasePDefChest:     31,
		BasePDefLegs:      18,
		BasePDefHead:      12,
		BasePDefFeet:      7,
		BasePDefGloves:    8,
		BasePDefUnderwear: 3,
		BasePDefCloak:     1,

		BaseMDefREar:    9,
		BaseMDefLEar:    9,
		BaseMDefRFinger: 5,
		BaseMDefLFinger: 5,
		BaseMDefNeck:    13,

		BaseRunSpd:      120,
		BaseWalkSpd:     50,
		BaseSwimRunSpd:  50,
		BaseSwimWalkSpd: 50,
	}
}

// mysticCombatStats returns combat base stats for mystic classes
// Values from pcBaseStats.xml: basePAtk=3, baseMAtk=6, baseCritRate=4,
// basePAtkSpd=300, baseMAtkSpd=333
// Mystic PDef: chest=15, legs=8, head=12, feet=7, gloves=8, underwear=3, cloak=1
func mysticCombatStats() models.CombatBaseStats {
	return models.CombatBaseStats{
		BasePAtk:    3,
		BaseMAtk:    6,
		BaseCritRate: 4,
		BasePAtkSpd: 300,
		BaseMAtkSpd: 333,

		BasePDefChest:     15,
		BasePDefLegs:      8,
		BasePDefHead:      12,
		BasePDefFeet:      7,
		BasePDefGloves:    8,
		BasePDefUnderwear: 3,
		BasePDefCloak:     1,

		BaseMDefREar:    9,
		BaseMDefLEar:    9,
		BaseMDefRFinger: 5,
		BaseMDefLFinger: 5,
		BaseMDefNeck:    13,

		BaseRunSpd:      120,
		BaseWalkSpd:     50,
		BaseSwimRunSpd:  50,
		BaseSwimWalkSpd: 50,
	}
}

// orcMysticCombatStats returns combat base stats for Orc Mystic (basePAtk=4 unlike other mystics)
func orcMysticCombatStats() models.CombatBaseStats {
	stats := mysticCombatStats()
	stats.BasePAtk = 4
	return stats
}

// getCharacterTemplate returns the template for a specific race/class combination
func (uc *CharacterUseCase) getCharacterTemplate(race, classID int) (*CharacterTemplate, error) {
	templates := getDefaultCharacterTemplates()

	for _, template := range templates {
		if template.Race == race && template.ClassID == classID {
			return &template, nil
		}
	}

	return nil, fmt.Errorf("no template found for race %d, class %d", race, classID)
}

// getDefaultCharacterTemplates returns the default L2J character templates
// Item IDs sourced from L2J initialEquipment.xml
func getDefaultCharacterTemplates() []CharacterTemplate {
	return []CharacterTemplate{
		// Human Fighter
		{
			Race:      int(models.RaceHuman),
			ClassID:   int(models.ClassHumanFighter),
			ClassName: "Human Fighter",
			BaseStats: CharacterBaseStats{
				STR: 40, DEX: 30, CON: 43, INT: 25, WIT: 11, MEN: 25,
				HP: 669, MP: 588, CP: 334,
				Combat: fighterCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml)
				{ItemID: 2369, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)}, // Squire's Sword
				{ItemID: 1146, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)}, // Squire's Shirt
				{ItemID: 1147, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},  // Squire's Pants
				// Starting consumables
				{ItemID: 736, Count: 100, Location: models.LocInventory, LocData: -1},  // SoP
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1177, Level: 1}, // Wind Strike (basic attack skill)
				{SkillID: 1184, Level: 1}, // Power Strike
				{SkillID: 1146, Level: 1}, // Stun Attack
			},
			StartingPosition: models.Position{X: -84318, Y: 244579, Z: -3730}, // Talking Island Village
		},

		// Human Mystic
		{
			Race:      int(models.RaceHuman),
			ClassID:   int(models.ClassHumanMystic),
			ClassName: "Human Mystic",
			BaseStats: CharacterBaseStats{
				STR: 25, DEX: 25, CON: 30, INT: 41, WIT: 35, MEN: 39,
				HP: 588, MP: 792, CP: 294,
				Combat: mysticCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml)
				{ItemID: 6, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)},    // Apprentice's Wand
				{ItemID: 425, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)},  // Apprentice's Tunic
				{ItemID: 461, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},   // Apprentice's Stockings
				// Starting consumables
				{ItemID: 728, Count: 100, Location: models.LocInventory, LocData: -1},  // SoS
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1005, Level: 1}, // Heal
				{SkillID: 1001, Level: 1}, // Might
				{SkillID: 1177, Level: 1}, // Wind Strike
			},
			StartingPosition: models.Position{X: -84318, Y: 244579, Z: -3730}, // Talking Island Village
		},

		// Elf Fighter
		{
			Race:      int(models.RaceElf),
			ClassID:   int(models.ClassElfFighter),
			ClassName: "Elf Fighter",
			BaseStats: CharacterBaseStats{
				STR: 36, DEX: 35, CON: 36, INT: 25, WIT: 14, MEN: 25,
				HP: 588, MP: 588, CP: 294,
				Combat: fighterCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml)
				{ItemID: 2369, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)}, // Squire's Sword
				{ItemID: 1146, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)}, // Squire's Shirt
				{ItemID: 1147, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},  // Squire's Pants
				// Starting consumables
				{ItemID: 736, Count: 100, Location: models.LocInventory, LocData: -1},  // SoP
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1177, Level: 1}, // Wind Strike
				{SkillID: 1184, Level: 1}, // Power Strike
				{SkillID: 1146, Level: 1}, // Stun Attack
			},
			StartingPosition: models.Position{X: 46934, Y: 51467, Z: -2977}, // Elf Village
		},

		// Elf Mystic
		{
			Race:      int(models.RaceElf),
			ClassID:   int(models.ClassElfMystic),
			ClassName: "Elf Mystic",
			BaseStats: CharacterBaseStats{
				STR: 25, DEX: 25, CON: 31, INT: 37, WIT: 35, MEN: 40,
				HP: 588, MP: 792, CP: 294,
				Combat: mysticCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml)
				{ItemID: 6, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)},    // Apprentice's Wand
				{ItemID: 425, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)},  // Apprentice's Tunic
				{ItemID: 461, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},   // Apprentice's Stockings
				// Starting consumables
				{ItemID: 728, Count: 100, Location: models.LocInventory, LocData: -1},  // SoS
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1005, Level: 1}, // Heal
				{SkillID: 1002, Level: 1}, // Shield
				{SkillID: 1177, Level: 1}, // Wind Strike
			},
			StartingPosition: models.Position{X: 46934, Y: 51467, Z: -2977}, // Elf Village
		},

		// Dark Elf Fighter
		{
			Race:      int(models.RaceDarkElf),
			ClassID:   int(models.ClassDarkElfFighter),
			ClassName: "Dark Elf Fighter",
			BaseStats: CharacterBaseStats{
				STR: 41, DEX: 32, CON: 32, INT: 25, WIT: 12, MEN: 25,
				HP: 588, MP: 588, CP: 294,
				Combat: fighterCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml)
				{ItemID: 2369, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)}, // Squire's Sword
				{ItemID: 1146, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)}, // Squire's Shirt
				{ItemID: 1147, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},  // Squire's Pants
				// Starting consumables
				{ItemID: 736, Count: 100, Location: models.LocInventory, LocData: -1},  // SoP
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1177, Level: 1}, // Wind Strike
				{SkillID: 1184, Level: 1}, // Power Strike
				{SkillID: 1146, Level: 1}, // Stun Attack
			},
			StartingPosition: models.Position{X: 28384, Y: 11056, Z: -4233}, // Dark Elf Village
		},

		// Dark Elf Mystic
		{
			Race:      int(models.RaceDarkElf),
			ClassID:   int(models.ClassDarkElfMystic),
			ClassName: "Dark Elf Mystic",
			BaseStats: CharacterBaseStats{
				STR: 25, DEX: 25, CON: 31, INT: 37, WIT: 35, MEN: 40,
				HP: 588, MP: 792, CP: 294,
				Combat: mysticCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml)
				{ItemID: 6, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)},    // Apprentice's Wand
				{ItemID: 425, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)},  // Apprentice's Tunic
				{ItemID: 461, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},   // Apprentice's Stockings
				// Starting consumables
				{ItemID: 728, Count: 100, Location: models.LocInventory, LocData: -1},  // SoS
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1005, Level: 1}, // Heal
				{SkillID: 1002, Level: 1}, // Shield
				{SkillID: 1177, Level: 1}, // Wind Strike
			},
			StartingPosition: models.Position{X: 28384, Y: 11056, Z: -4233}, // Dark Elf Village
		},

		// Orc Fighter
		{
			Race:      int(models.RaceOrc),
			ClassID:   int(models.ClassOrcFighter),
			ClassName: "Orc Fighter",
			BaseStats: CharacterBaseStats{
				STR: 40, DEX: 26, CON: 47, INT: 25, WIT: 12, MEN: 27,
				HP: 669, MP: 588, CP: 334,
				Combat: fighterCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml - Orcs use Training Gloves)
				{ItemID: 2369, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)}, // Squire's Sword
				{ItemID: 1146, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)}, // Squire's Shirt
				{ItemID: 1147, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},  // Squire's Pants
				// Starting consumables
				{ItemID: 736, Count: 100, Location: models.LocInventory, LocData: -1},  // SoP
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1177, Level: 1}, // Wind Strike
				{SkillID: 1184, Level: 1}, // Power Strike
				{SkillID: 1146, Level: 1}, // Stun Attack
			},
			StartingPosition: models.Position{X: -45186, Y: -112459, Z: -236}, // Orc Village
		},

		// Orc Mystic
		{
			Race:      int(models.RaceOrc),
			ClassID:   int(models.ClassOrcMystic),
			ClassName: "Orc Mystic",
			BaseStats: CharacterBaseStats{
				STR: 25, DEX: 25, CON: 34, INT: 37, WIT: 35, MEN: 42,
				HP: 588, MP: 792, CP: 294,
				Combat: orcMysticCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml)
				{ItemID: 6, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)},    // Apprentice's Wand
				{ItemID: 425, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)},  // Apprentice's Tunic
				{ItemID: 461, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},   // Apprentice's Stockings
				// Starting consumables
				{ItemID: 728, Count: 100, Location: models.LocInventory, LocData: -1},  // SoS
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1005, Level: 1}, // Heal
				{SkillID: 1002, Level: 1}, // Shield
				{SkillID: 1177, Level: 1}, // Wind Strike
			},
			StartingPosition: models.Position{X: -45186, Y: -112459, Z: -236}, // Orc Village
		},

		// Dwarf Fighter
		{
			Race:      int(models.RaceDwarf),
			ClassID:   int(models.ClassDwarfFighter),
			ClassName: "Dwarf Fighter",
			BaseStats: CharacterBaseStats{
				STR: 39, DEX: 29, CON: 45, INT: 25, WIT: 11, MEN: 25,
				HP: 669, MP: 588, CP: 334,
				Combat: fighterCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml - Dwarves use Guild Member's Club)
				{ItemID: 2370, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)}, // Guild Member's Club
				{ItemID: 1146, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)}, // Squire's Shirt
				{ItemID: 1147, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},  // Squire's Pants
				// Starting consumables
				{ItemID: 736, Count: 100, Location: models.LocInventory, LocData: -1},  // SoP
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1177, Level: 1}, // Wind Strike
				{SkillID: 1184, Level: 1}, // Power Strike
				{SkillID: 1146, Level: 1}, // Stun Attack
			},
			StartingPosition: models.Position{X: 108512, Y: -173026, Z: -406}, // Dwarf Village
		},

		// Kamael Soldier
		{
			Race:      int(models.RaceKamael),
			ClassID:   int(models.ClassKamaelSoldier),
			ClassName: "Kamael Soldier",
			BaseStats: CharacterBaseStats{
				STR: 40, DEX: 33, CON: 35, INT: 25, WIT: 12, MEN: 25,
				HP: 588, MP: 588, CP: 294,
				Combat: fighterCombatStats(),
			},
			StartingItems: []StartingItemData{
				// Basic equipment (L2J initialEquipment.xml)
				{ItemID: 2369, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotRHand)}, // Squire's Sword
				{ItemID: 1146, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotChest)}, // Squire's Shirt
				{ItemID: 1147, Count: 1, Location: models.LocPaperdoll, LocData: int(models.SlotLegs)},  // Squire's Pants
				// Starting consumables
				{ItemID: 736, Count: 100, Location: models.LocInventory, LocData: -1},  // SoP
				{ItemID: 1060, Count: 100, Location: models.LocInventory, LocData: -1}, // Lesser Healing Potion
			},
			StartingSkills: []StartingSkillData{
				{SkillID: 1177, Level: 1}, // Wind Strike
				{SkillID: 1184, Level: 1}, // Power Strike
				{SkillID: 1146, Level: 1}, // Stun Attack
			},
			StartingPosition: models.Position{X: -125740, Y: 38016, Z: 1251}, // Kamael Village
		},
	}
}

// Race and class validation functions

// isValidRace checks if race ID is valid
func isValidRace(race int) bool {
	return race >= int(models.RaceHuman) && race <= int(models.RaceKamael)
}

// isValidSex checks if sex ID is valid
func isValidSex(sex int) bool {
	return sex == int(models.SexMale) || sex == int(models.SexFemale)
}

// isValidClassForRace checks if class is valid for the given race
func isValidClassForRace(race, classID int) bool {
	validClasses := map[int][]int{
		int(models.RaceHuman):   {int(models.ClassHumanFighter), int(models.ClassHumanMystic)},
		int(models.RaceElf):     {int(models.ClassElfFighter), int(models.ClassElfMystic)},
		int(models.RaceDarkElf): {int(models.ClassDarkElfFighter), int(models.ClassDarkElfMystic)},
		int(models.RaceOrc):     {int(models.ClassOrcFighter), int(models.ClassOrcMystic)},
		int(models.RaceDwarf):   {int(models.ClassDwarfFighter)},
		int(models.RaceKamael):  {int(models.ClassKamaelSoldier)},
	}

	classes, exists := validClasses[race]
	if !exists {
		return false
	}

	for _, validClass := range classes {
		if classID == validClass {
			return true
		}
	}

	return false
}

// GetCombatBaseStatsByClass returns combat base stats for a given class ID.
// This is used by handlers to compute stats when building packets.
func GetCombatBaseStatsByClass(classID int) models.CombatBaseStats {
	templates := getDefaultCharacterTemplates()
	for _, t := range templates {
		if t.ClassID == classID {
			return t.BaseStats.Combat
		}
	}
	// Fallback to fighter stats
	return fighterCombatStats()
}
