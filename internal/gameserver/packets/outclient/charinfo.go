package outclient

import (
	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// computeCharInfoStats computes combat and movement stats for CharInfo from character base stats.
// Uses the same stat calculation as UserInfo for consistency.
func computeCharInfoStats(char *models.Character) models.ComputedStats {
	baseStats := models.CharacterStats{
		STR: char.BaseSTR,
		DEX: char.BaseDEX,
		CON: char.BaseCON,
		INT: char.BaseINT,
		WIT: char.BaseWIT,
		MEN: char.BaseMEN,
	}
	// Look up combat base stats by class — avoid importing usecase to prevent circular dependency
	// Use a simple fighter/mystic heuristic based on class ID
	combat := defaultCombatBaseStats(char.ClassID)
	computed := models.ComputeStats(baseStats, char.Level, combat)
	return models.ApplyStatModifiers(computed, char.StatMods)
}

// defaultCombatBaseStats returns combat base stats for a class ID without importing usecase.
// This avoids circular dependency between packets/outclient and usecase.
func defaultCombatBaseStats(classID int) models.CombatBaseStats {
	// Mystic class IDs: HumanMystic=10, ElfMystic=25, DarkElfMystic=38, OrcMystic=49
	isMystic := classID == 10 || classID == 25 || classID == 38 || classID == 49

	if isMystic {
		stats := models.CombatBaseStats{
			BasePAtk: 3, BaseMAtk: 6, BaseCritRate: 4,
			BasePAtkSpd: 300, BaseMAtkSpd: 333,
			BasePDefChest: 15, BasePDefLegs: 8, BasePDefHead: 12,
			BasePDefFeet: 7, BasePDefGloves: 8, BasePDefUnderwear: 3, BasePDefCloak: 1,
			BaseMDefREar: 9, BaseMDefLEar: 9, BaseMDefRFinger: 5, BaseMDefLFinger: 5, BaseMDefNeck: 13,
			BaseRunSpd: 120, BaseWalkSpd: 50, BaseSwimRunSpd: 50, BaseSwimWalkSpd: 50,
		}
		if classID == 49 { // Orc Mystic has BasePAtk=4
			stats.BasePAtk = 4
		}
		return stats
	}
	// Fighter stats (default)
	return models.CombatBaseStats{
		BasePAtk: 4, BaseMAtk: 6, BaseCritRate: 4,
		BasePAtkSpd: 300, BaseMAtkSpd: 333,
		BasePDefChest: 31, BasePDefLegs: 18, BasePDefHead: 12,
		BasePDefFeet: 7, BasePDefGloves: 8, BasePDefUnderwear: 3, BasePDefCloak: 1,
		BaseMDefREar: 9, BaseMDefLEar: 9, BaseMDefRFinger: 5, BaseMDefLFinger: 5, BaseMDefNeck: 13,
		BaseRunSpd: 120, BaseWalkSpd: 50, BaseSwimRunSpd: 50, BaseSwimWalkSpd: 50,
	}
}

// CharInfo packet (0x31) - shows OTHER players to the current player
// This is different from UserInfo (0x32) which shows the player's own info
type CharInfo struct {
	// Position and identity
	X        int32
	Y        int32
	Z        int32
	Heading  int32
	ObjectID int32
	Name     string
	Race     int32
	Sex      int32 // 1 for male, 0 for female
	ClassID  int32

	// Equipment - only visual display (no object IDs needed for other players)
	PaperdollDisplayIDs [26]int32 // Visual appearance item IDs
	PaperdollAugmentIDs [26]int32 // Enchant/augment effect IDs

	// Equipment capabilities
	TalismanSlots int32
	CanEquipCloak int32

	// PvP and status
	PvPFlag int32
	Karma   int32

	// Combat stats (basic for other players)
	MAtkSpd int32
	PAtkSpd int32

	// Movement speeds
	RunSpd      int32
	WalkSpd     int32
	SwimRunSpd  int32
	SwimWalkSpd int32
	FlRunSpd    int32
	FlWalkSpd   int32

	// Appearance
	HairStyle int32
	HairColor int32
	Face      int32

	// Title and clan
	Title     string
	ClanID    int32
	ClanCrest int32
	AllyID    int32
	AllyCrest int32

	// Status flags
	Sitting   int32 // 0 = standing, 1 = sitting
	Running   int32 // 0 = walking, 1 = running
	InCombat  int32 // 0 = peaceful, 1 = in combat
	Dead      int32 // 0 = alive, 1 = dead
	Invisible int32 // 0 = visible, 1 = invisible

	// Mount and store
	MountType        int32
	PrivateStoreType int32

	// Cubics
	Cubics []int32

	// Additional status
	PartyFlag    int32 // 0 = not in party match room, 1 = in party match room
	AbnormalMask int32 // Visual effects mask
	Zone         int32 // 0 = ground, 1 = water, 2 = flying

	// Recommendations
	RecomHave int32

	// Mount NPC
	MountNPCID int32

	// Class and effects
	ClassId2     int32
	EnchantLevel int32
	TeamID       int32

	// Large clan crest
	LargeClanCrest int32

	// Noble and hero status
	Noble int32 // 0 = normal, 1 = noble
	Hero  int32 // 0 = normal, 1 = hero

	// Fishing info
	FishingFlag int32
	FishingX    int32
	FishingY    int32
	FishingZ    int32

	// Name and title colors
	NameColor  int32
	TitleColor int32

	// Pledge info
	PledgeClass int32
	PledgeType  int32

	// Advanced status
	CursedWeaponLevel   int32
	ClanReputation      int32
	TransformDisplayID  int32
	AgathionID          int32
	SpecialAbnormalMask int32
}

// BuildCharInfo creates CharInfo packet data using pkg/l2pkt
// Based on Java L2J CharInfo packet structure (normal character version)
func BuildCharInfo(info CharInfo) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x31) // CharInfo opcode for normal characters

	collisionRadius, collisionHeight := getCollisionValues(info.Race, info.Sex)

	// Position and identity
	w.WriteD(info.X)
	w.WriteD(info.Y)
	w.WriteD(info.Z)
	w.WriteD(0) // Vehicle ID (always 0 for normal characters)
	w.WriteD(info.ObjectID)
	w.WriteS(info.Name)
	w.WriteD(info.Race)
	w.WriteD(info.Sex)
	w.WriteD(info.ClassID)

	// Equipment - Two loops using PAPERDOLL_ORDER (EXACTLY 21 elements like Java L2J)
	// Based on Java L2J CharInfo.java lines 49-71
	// Uses Inventory.PAPERDOLL_* slot indices
	paperdollOrder := []int{
		0,  // PAPERDOLL_UNDER
		1,  // PAPERDOLL_HEAD
		5,  // PAPERDOLL_RHAND
		7,  // PAPERDOLL_LHAND
		10, // PAPERDOLL_GLOVES
		6,  // PAPERDOLL_CHEST
		11, // PAPERDOLL_LEGS
		12, // PAPERDOLL_FEET
		23, // PAPERDOLL_CLOAK
		5,  // PAPERDOLL_RHAND (duplicate!)
		2,  // PAPERDOLL_HAIR
		3,  // PAPERDOLL_HAIR2
		16, // PAPERDOLL_RBRACELET
		15, // PAPERDOLL_LBRACELET
		17, // PAPERDOLL_DECO1
		18, // PAPERDOLL_DECO2
		19, // PAPERDOLL_DECO3
		20, // PAPERDOLL_DECO4
		21, // PAPERDOLL_DECO5
		22, // PAPERDOLL_DECO6
		24, // PAPERDOLL_BELT
	}

	// Loop 1: Display IDs (visual appearance item IDs)
	for _, slotIndex := range paperdollOrder {
		if slotIndex >= 0 && slotIndex < 26 {
			w.WriteD(info.PaperdollDisplayIDs[slotIndex])
		} else {
			w.WriteD(0)
		}
	}

	// Loop 2: Augmentation IDs (enchant/augment effect IDs)
	for _, slotIndex := range paperdollOrder {
		if slotIndex >= 0 && slotIndex < 26 {
			w.WriteD(info.PaperdollAugmentIDs[slotIndex])
		} else {
			w.WriteD(0)
		}
	}

	// Equipment capabilities
	w.WriteD(info.TalismanSlots)
	w.WriteD(info.CanEquipCloak)

	// PvP status
	w.WriteD(info.PvPFlag)
	w.WriteD(info.Karma)

	// Combat stats
	w.WriteD(info.MAtkSpd)
	w.WriteD(info.PAtkSpd)
	w.WriteD(0) // Unknown field

	// Movement speeds (Java L2J has intentional duplicate fly speeds)
	w.WriteD(info.RunSpd)
	w.WriteD(info.WalkSpd)
	w.WriteD(info.SwimRunSpd)
	w.WriteD(info.SwimWalkSpd)
	w.WriteD(info.FlRunSpd)
	w.WriteD(info.FlWalkSpd)
	w.WriteD(info.FlRunSpd)  // Intentional duplicate (Java L2J)
	w.WriteD(info.FlWalkSpd) // Intentional duplicate (Java L2J)

	// Movement and attack speed multipliers
	w.WriteF(1.0) // Movement multiplier
	w.WriteF(1.0) // Attack speed multiplier

	// Collision radius and height (race/gender specific)
	w.WriteF(float64(collisionRadius))
	w.WriteF(float64(collisionHeight))

	// Appearance
	w.WriteD(info.HairStyle)
	w.WriteD(info.HairColor)
	w.WriteD(info.Face)

	// Title and clan info
	w.WriteS(info.Title)
	w.WriteD(info.ClanID)
	w.WriteD(info.ClanCrest)
	w.WriteD(info.AllyID)
	w.WriteD(info.AllyCrest)

	// Status flags
	// L2J writes standing = 1, sitting = 0 (writeC(isSitting ? 0 : 1)), so a
	// standing player MUST encode as 1 — writing the raw 0 makes the client render
	// everyone seated on the ground.
	if info.Sitting != 0 {
		w.WriteC(0) // sitting
	} else {
		w.WriteC(1) // standing
	}
	w.WriteC(uint8(info.Running))   // Running flag
	w.WriteC(uint8(info.InCombat))  // In combat flag
	w.WriteC(uint8(info.Dead))      // Dead flag
	w.WriteC(uint8(info.Invisible)) // Invisible flag

	// Mount and store info
	w.WriteC(uint8(info.MountType))        // Mount type
	w.WriteC(uint8(info.PrivateStoreType)) // Private store type

	// Cubics
	w.WriteH(uint16(len(info.Cubics)))
	for _, cubic := range info.Cubics {
		w.WriteH(uint16(cubic))
	}

	// Additional status
	w.WriteC(uint8(info.PartyFlag)) // In party match room

	// Abnormal effects and zone info
	w.WriteD(info.AbnormalMask)
	w.WriteC(uint8(info.Zone)) // Zone (water/flying)

	// Recommendations
	w.WriteH(uint16(info.RecomHave))

	// Mount NPC ID
	w.WriteD(info.MountNPCID)

	// Class and effects
	w.WriteD(info.ClassId2)
	w.WriteD(0) // Unknown field
	w.WriteC(uint8(info.EnchantLevel))

	// Team and crests
	w.WriteC(uint8(info.TeamID))
	w.WriteD(info.LargeClanCrest)
	w.WriteC(uint8(info.Noble))
	w.WriteC(uint8(info.Hero))

	// Fishing info — L2J writes the fishing flag as a single byte (writeC). Writing
	// it as a 4-byte D shifts every following field (name color, heading, title
	// color, ...) by 3 bytes, so the client reads garbage colors/heading.
	w.WriteC(uint8(info.FishingFlag))
	w.WriteD(info.FishingX)
	w.WriteD(info.FishingY)
	w.WriteD(info.FishingZ)

	// Name and title colors
	w.WriteD(info.NameColor)

	// Heading
	w.WriteD(info.Heading)

	// Pledge info
	w.WriteD(info.PledgeClass)
	w.WriteD(info.PledgeType)

	// Title color
	w.WriteD(info.TitleColor)

	// Advanced status
	w.WriteD(info.CursedWeaponLevel)
	w.WriteD(info.ClanReputation)
	w.WriteD(info.TransformDisplayID)
	w.WriteD(info.AgathionID)
	w.WriteD(1) // Unknown field (always 1 in Java L2J)
	w.WriteD(info.SpecialAbnormalMask)

	return w.Bytes()
}

// NewCharInfo creates a CharInfo packet from character and player state data.
// paperdollDisplayIDs are the visible equipment item IDs per paperdoll slot (from
// the cached char.PaperdollItems, so no DB lookup is needed). heading is the
// character's live facing (from the world registry); passing 0 makes every player
// face north.
func NewCharInfo(char *models.Character, playerState *models.Position, paperdollDisplayIDs [26]int32, isRunning bool, inCombat bool, heading int32) *CharInfo {
	// Compute stats once for this character
	computed := computeCharInfoStats(char)

	charInfo := &CharInfo{
		// Position from player state
		X:        int32(playerState.X),
		Y:        int32(playerState.Y),
		Z:        int32(playerState.Z),
		Heading:  heading,
		ObjectID: char.ID,
		Name:     char.Name,

		// Character attributes
		Race:    int32(char.Race),
		Sex:     int32(char.Sex),
		ClassID: int32(char.BaseClass),

		// Equipment capabilities
		TalismanSlots: 0, // TODO: Calculate based on character level/quest
		CanEquipCloak: 1,

		// PvP status
		PvPFlag: 0, // TODO: Get from character PvP state
		Karma:   int32(char.Karma),

		// Combat stats computed from character base stats
		MAtkSpd: int32(computed.MAtkSpd),
		PAtkSpd: int32(computed.PAtkSpd),

		// Movement speeds computed from character base stats
		RunSpd:      int32(computed.RunSpd),
		WalkSpd:     int32(computed.WalkSpd),
		SwimRunSpd:  int32(computed.SwimRunSpd),
		SwimWalkSpd: int32(computed.SwimWalkSpd),
		FlRunSpd:    0,
		FlWalkSpd:   0,

		// Appearance from character model
		HairStyle: int32(char.HairStyle),
		HairColor: int32(char.HairColor),
		Face:      int32(char.Face),

		// Title and clan
		Title:     char.Title,
		ClanID:    int32(char.ClanID),
		ClanCrest: 0, // TODO: Get clan crest ID
		AllyID:    0, // TODO: Get ally ID
		AllyCrest: 0, // TODO: Get ally crest ID

		// Status flags
		Sitting:   0, // 0 = standing
		Running:   func() int32 { if isRunning { return 1 } else { return 0 } }(), // Running state from parameter
		InCombat:  func() int32 { if inCombat { return 1 } else { return 0 } }(), // Combat state from parameter
		Dead:      0, // 0 = alive
		Invisible: 0, // 0 = visible

		// Mount and store
		MountType:        0, // 0 = no mount
		PrivateStoreType: 0, // 0 = no private store

		// Cubics (empty for now)
		Cubics: []int32{},

		// Additional status
		PartyFlag:    0, // 0 = not in party match room
		AbnormalMask: 0, // No visual effects
		Zone:         0, // 0 = ground

		// Recommendations
		RecomHave: 0, // TODO: Get recommendation count

		// Mount NPC
		MountNPCID: 0,

		// Class and effects
		ClassId2:     int32(char.BaseClass),
		EnchantLevel: 0, // TODO: Get enchant effect
		TeamID:       0, // 0 = no team

		// Large clan crest
		LargeClanCrest: 0, // TODO: Get large clan crest

		// Noble and hero status
		Noble: 0, // TODO: Check noble status
		Hero:  0, // TODO: Check hero status

		// Fishing info (not fishing)
		FishingFlag: 0,
		FishingX:    0,
		FishingY:    0,
		FishingZ:    0,

		// Name and title colors (default)
		NameColor:  0xFFFFFF, // White
		TitleColor: 0xFFFF00, // Yellow

		// Pledge info (TODO: Get from clan data)
		PledgeClass: 0,
		PledgeType:  0,

		// Advanced status
		CursedWeaponLevel:   0,
		ClanReputation:      0,
		TransformDisplayID:  0,
		AgathionID:          0,
		SpecialAbnormalMask: 0,
	}

	// Visible equipment for display, taken from the cached paperdoll (no DB lookup).
	// Augment display IDs are not cached yet — left zero (no augment glow). (l2go-23g)
	charInfo.PaperdollDisplayIDs = paperdollDisplayIDs

	return charInfo
}

// GetData returns the packet data bytes
func (p *CharInfo) GetData() []byte {
	return BuildCharInfo(*p)
}

// getCollisionValues returns collision radius and height based on race and gender
// Based on Java L2J collision values for different races
func getCollisionValues(race int32, sex int32) (float32, float32) {
	// Race constants (should match your character creation system)
	const (
		RaceHuman   = 0
		RaceElf     = 1
		RaceDarkElf = 2
		RaceOrc     = 3
		RaceDwarf   = 4
	)

	// Gender constants: 1 = male, 0 = female
	isMale := sex == 1

	switch race {
	case RaceHuman:
		if isMale {
			return 8.0, 25.0 // Human Male - increased height
		} else {
			return 7.0, 24.0 // Human Female - increased height
		}
	case RaceElf:
		if isMale {
			return 7.0, 26.0 // Elf Male - increased height
		} else {
			return 6.0, 25.0 // Elf Female - increased height
		}
	case RaceDarkElf:
		if isMale {
			return 7.0, 26.0 // Dark Elf Male - increased height
		} else {
			return 6.0, 25.0 // Dark Elf Female - increased height
		}
	case RaceOrc:
		if isMale {
			return 11.0, 30.0 // Orc Male - increased height
		} else {
			return 8.5, 28.0 // Orc Female - increased height
		}
	case RaceDwarf:
		if isMale {
			return 7.0, 20.0 // Dwarf Male - increased height
		} else {
			return 6.0, 19.0 // Dwarf Female - increased height
		}
	default:
		// Fallback to Human Male values
		return 8.0, 25.0
	}
}
