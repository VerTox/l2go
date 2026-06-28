package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// UserInfo packet (opcode 0x32) - comprehensive character information for world entry
type UserInfo struct {
	// Character identity
	X        int32
	Y        int32
	Z        int32
	ObjectID int32
	Name     string
	Race     int32
	Sex      int32
	ClassID  int32

	// Character stats
	Level int32
	EXP   int64
	STR   int32
	DEX   int32
	CON   int32
	INT   int32
	WIT   int32
	MEN   int32

	// Current status
	MaxHP     int32
	CurrentHP int32
	MaxMP     int32
	CurrentMP int32
	MaxCP     int32
	CurrentCP int32
	CurrentSP int64

	// Combat stats
	PAtk     int32
	AtkSpd   int32
	PDef     int32
	Evasion  int32
	Accuracy int32
	Critical int32
	MAtk     int32
	CastSpd  int32
	MDef     int32

	// PvP info
	PvPFlag     int32
	Karma       int32
	RunSpd      int32
	WalkSpd     int32
	SwimRunSpd  int32
	SwimWalkSpd int32
	FlRunSpd    int32
	FlWalkSpd   int32
	FlyRunSpd   int32
	FlyWalkSpd  int32

	// Additional status
	Title     string
	ClanID    int32
	ClanCrest int32
	AllyID    int32
	AllyCrest int32

	// Combat state
	SittingFlag int32
	RunningFlag int32
	InCombat    int32
	Deceased    int32
	Invisible   int32

	// Mount info
	MountType int32
	MountID   int32

	// Private store
	PrivateStoreType int32
	CanCraft         int32

	// Additional stats
	PKKills      int32
	PVPKills     int32
	Cubics       []int32
	PartyFlag    int32
	AbnormalMask int32
	ClanPrivs    int32

	// Misc
	RecomLeft      int32
	RecomHave      int32
	InventoryLimit int32
	ClassId2       int32
	ExpPercent     float64
	CurrentLoad    int32
	MaxLoad        int32

	// Vehicle and equipment
	VehicleID int32

	// Paperdoll data - loaded from equipped items
	Paperdoll *PaperdollInfo

	// Equipment capabilities
	TalismanSlots int32
	CanEquipCloak int32

	// T2 Additional fields
	Fame           int32
	MinimapAllowed int32
	VitalityPoints int32
	SpecialEffects int32

	// Appearance
	HairStyle int32
	HairColor int32
	Face      int32

	// Collision
	CollisionRadius float64
	CollisionHeight float64
}

// PaperdollInfo holds equipment data for UserInfo packet
type PaperdollInfo struct {
	ObjectIDs  [26]int32 // Database object IDs of equipped items
	DisplayIDs [26]int32 // Visual item IDs for rendering
	AugmentIDs [26]int32 // Augmentation effect IDs
}

// UserInfoPaperdollOrder defines the order slots are sent in UserInfo packet
// This matches Java L2J L2GameServerPacket.PAPERDOLL_ORDER
var UserInfoPaperdollOrder = []int{
	0,  // PAPERDOLL_UNDER
	8,  // PAPERDOLL_REAR (right ear)
	9,  // PAPERDOLL_LEAR (left ear)
	4,  // PAPERDOLL_NECK
	13, // PAPERDOLL_RFINGER
	14, // PAPERDOLL_LFINGER
	1,  // PAPERDOLL_HEAD
	5,  // PAPERDOLL_RHAND
	7,  // PAPERDOLL_LHAND
	10, // PAPERDOLL_GLOVES
	6,  // PAPERDOLL_CHEST
	11, // PAPERDOLL_LEGS
	12, // PAPERDOLL_FEET
	23, // PAPERDOLL_CLOAK
	5,  // PAPERDOLL_RHAND (duplicate - intentional in L2J)
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

// NewPaperdollInfo creates empty paperdoll info
func NewPaperdollInfo() *PaperdollInfo {
	return &PaperdollInfo{}
}

// BuildUserInfo creates UserInfo packet data using pkg/l2pkt
// Java L2J compatible structure with proper paperdoll handling
func BuildUserInfo(info UserInfo) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x32) // UserInfo opcode

	// Position and identity (Java L2J lines 78-82)
	w.WriteD(info.X)
	w.WriteD(info.Y)
	w.WriteD(info.Z)
	w.WriteD(info.VehicleID) // Vehicle ID (0 if not in vehicle)
	w.WriteD(info.ObjectID)
	w.WriteS(info.Name)

	// Physical attributes
	w.WriteD(info.Race)
	w.WriteD(info.Sex)
	w.WriteD(info.ClassID)

	// Level and experience
	w.WriteD(info.Level)
	w.WriteQ(info.EXP)
	w.WriteF(info.ExpPercent)

	// Base attributes
	w.WriteD(info.STR)
	w.WriteD(info.DEX)
	w.WriteD(info.CON)
	w.WriteD(info.INT)
	w.WriteD(info.WIT)
	w.WriteD(info.MEN)

	// Health and mana
	w.WriteD(info.MaxHP)
	w.WriteD(info.CurrentHP)
	w.WriteD(info.MaxMP)
	w.WriteD(info.CurrentMP)

	// Skill points and load
	w.WriteD(int32(info.CurrentSP))
	w.WriteD(info.CurrentLoad)
	w.WriteD(info.MaxLoad)

	// Active weapon flag (20 = no weapon, 40 = weapon equipped)
	// Check PAPERDOLL_RHAND (slot 5) for weapon
	hasWeapon := false
	if info.Paperdoll != nil && info.Paperdoll.ObjectIDs[5] != 0 {
		hasWeapon = true
	}
	if hasWeapon {
		w.WriteD(40)
	} else {
		w.WriteD(20)
	}

	// Paperdoll - Three loops using PAPERDOLL_ORDER
	// Loop 1: ObjectIDs (database item instance IDs)
	for _, slotIndex := range UserInfoPaperdollOrder {
		if info.Paperdoll != nil && slotIndex < 26 {
			w.WriteD(info.Paperdoll.ObjectIDs[slotIndex])
		} else {
			w.WriteD(0)
		}
	}

	// Loop 2: DisplayIDs (visual item IDs for rendering)
	for _, slotIndex := range UserInfoPaperdollOrder {
		if info.Paperdoll != nil && slotIndex < 26 {
			w.WriteD(info.Paperdoll.DisplayIDs[slotIndex])
		} else {
			w.WriteD(0)
		}
	}

	// Loop 3: AugmentationIDs (enchant/augment effect IDs)
	for _, slotIndex := range UserInfoPaperdollOrder {
		if info.Paperdoll != nil && slotIndex < 26 {
			w.WriteD(info.Paperdoll.AugmentIDs[slotIndex])
		} else {
			w.WriteD(0)
		}
	}

	// Talisman slots and cloak capability (Java L2J lines 121-122)
	w.WriteD(info.TalismanSlots)
	w.WriteD(info.CanEquipCloak)

	// Combat stats
	w.WriteD(info.PAtk)
	w.WriteD(info.AtkSpd) // PAtkSpd first time
	w.WriteD(info.PDef)
	w.WriteD(info.Evasion)
	w.WriteD(info.Accuracy)
	w.WriteD(info.Critical)
	w.WriteD(info.MAtk)
	w.WriteD(info.CastSpd) // MAtkSpd
	w.WriteD(info.AtkSpd)  // PAtkSpd second time (Java L2J intentional duplicate)
	w.WriteD(info.MDef)

	// PvP status
	w.WriteD(info.PvPFlag)
	w.WriteD(info.Karma)

	// Movement speeds (Java L2J has intentional duplicate fly speeds)
	w.WriteD(info.RunSpd)
	w.WriteD(info.WalkSpd)
	w.WriteD(info.SwimRunSpd)
	w.WriteD(info.SwimWalkSpd)
	w.WriteD(info.FlyRunSpd)
	w.WriteD(info.FlyWalkSpd)
	w.WriteD(info.FlyRunSpd)  // Intentional duplicate (Java L2J)
	w.WriteD(info.FlyWalkSpd) // Intentional duplicate (Java L2J)

	// Movement and attack speed multipliers
	w.WriteF(1.0) // Movement multiplier
	w.WriteF(1.0) // Attack speed multiplier

	// Collision radius and height
	w.WriteF(info.CollisionRadius)
	w.WriteF(info.CollisionHeight)

	// Appearance
	w.WriteD(info.HairStyle)
	w.WriteD(info.HairColor)
	w.WriteD(info.Face)
	w.WriteD(0) // isGM

	// Title and clan info
	w.WriteS(info.Title)
	w.WriteD(info.ClanID)
	w.WriteD(info.ClanCrest)
	w.WriteD(info.AllyID)
	w.WriteD(info.AllyCrest)
	w.WriteD(0) // Relation

	// Mount and store info
	w.WriteC(0) // Mount type
	w.WriteC(0) // Private store type
	w.WriteC(0) // Can craft

	// PK/PvP kills
	w.WriteD(info.PKKills)
	w.WriteD(info.PVPKills)

	// Cubics
	w.WriteH(uint16(len(info.Cubics)))
	for _, cubic := range info.Cubics {
		w.WriteH(uint16(cubic))
	}

	// Party flag
	w.WriteC(0) // In party match room

	// Abnormal effects and zone info
	w.WriteD(info.AbnormalMask)
	w.WriteC(0) // Zone (water/flying)

	// Clan privileges
	w.WriteD(info.ClanPrivs)

	// Recommendations
	w.WriteH(uint16(info.RecomLeft))
	w.WriteH(uint16(info.RecomHave))
	w.WriteD(0) // Mount NPC ID
	w.WriteH(uint16(info.InventoryLimit))

	// Class and CP
	w.WriteD(info.ClassId2)
	w.WriteD(0x00) // Special effects
	w.WriteD(info.MaxCP)
	w.WriteD(info.CurrentCP)
	w.WriteC(0) // Enchant effect

	// Team and crests
	w.WriteC(0) // Team ID

	w.WriteD(0) // Large clan crest
	w.WriteC(0) // Noble status
	w.WriteC(0) // Hero status

	w.WriteC(0) // Fishing mode
	w.WriteD(0) // Fish X
	w.WriteD(0) // Fish Y
	w.WriteD(0) // Fish Z
	w.WriteD(0xFFFFFF) // Name color (white, L2J default)

	w.WriteC(byte(info.RunningFlag))

	w.WriteD(0) // Pledge class
	w.WriteD(0) // Pledge type

	w.WriteD(0xECF9A2) // Title color (light green, L2J DEFAULT_TITLE_COLOR)

	w.WriteD(0) // Cursed weapon level

	// T1 Starts
	w.WriteD(0) // Transformation ID

	w.WriteH(0)  // Attack attribute
	w.WriteH(0)  // Attack attribute value
	w.WriteH(10) // Fire defense
	w.WriteH(10) // Water defense
	w.WriteH(10) // Wind defense
	w.WriteH(10) // Earth defense
	w.WriteH(10) // Holy defense
	w.WriteH(10) // Dark defense

	w.WriteD(0) // Agathion ID

	// T2 Additional fields
	w.WriteD(info.Fame)
	w.WriteD(info.MinimapAllowed)
	w.WriteD(info.VitalityPoints)
	w.WriteD(info.SpecialEffects)

	return w.Bytes()
}
