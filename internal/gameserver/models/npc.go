package models

// NpcTemplate holds the static data for an NPC type loaded from XML.
type NpcTemplate struct {
	ID        int32
	DisplayID int32  // usually == ID
	Name      string
	Title     string
	Level     int
	Type      string // "L2Npc", "L2Monster", "L2Guard", etc.
	Race      string // "HUMAN", "ANIMAL", etc.
	Sex       string

	// Vitals
	HP float64
	MP float64

	// Attack
	PAtk    float64
	MAtk    float64
	PAtkSpd int
	MAtkSpd int

	// Defence
	PDef float64
	MDef float64

	// Speed
	RunSpd  int
	WalkSpd int

	// Combat misc
	CritRate    int
	AttackRange int

	// Rewards (from datapack <acquire expRate=".." sp=".."/>). RewardExp is the base
	// EXP = level² × expRate (L2J getExpReward before server rate); RewardSp is the
	// raw sp value. Both 0 for NPCs with no <acquire> (non-killable / no reward).
	RewardExp int64
	RewardSp  int64

	// Equipment visuals (3 slots)
	RHand int32
	LHand int32
	Chest int32

	// Collision
	CollisionRadius float64
	CollisionHeight float64

	// Flags
	Attackable bool
	Targetable bool
	ShowName   bool
	CanMove    bool
	AggroRange int
}

// NpcInstance represents a live NPC spawned in the game world.
type NpcInstance struct {
	ObjectID   int32        // unique runtime object ID
	TemplateID int32        // NPC template ID
	Template   *NpcTemplate // cached template pointer
	Position   Position
	Heading    int32
	IsRunning  bool
	IsDead     bool
	CurrentHP  float64
	CurrentMP  float64
	SpawnID    int32 // which spawn point created this NPC
}

// IsAttackable returns true if this NPC should be attacked on interaction
// rather than opening a dialogue window (monsters, raid bosses, guards, etc.).
func (n *NpcInstance) IsAttackable() bool {
	if n.Template == nil {
		return false
	}
	switch n.Template.Type {
	case "L2Monster", "L2RaidBoss", "L2GrandBoss",
		"L2Attackable", "L2FeedableBeast", "L2Guard",
		"L2Chest", "L2MonsterInstance":
		return true
	default:
		return false
	}
}

// WorldObject interface implementation for NpcInstance

func (n *NpcInstance) GetObjectID() int32      { return n.ObjectID }
func (n *NpcInstance) GetPosition() Position    { return n.Position }
func (n *NpcInstance) GetName() string          { return n.Template.Name }
func (n *NpcInstance) GetObjectType() ObjectType { return ObjectTypeNPC }

// SpawnData represents a spawn point from XML data.
type SpawnData struct {
	NpcID        int32
	X, Y, Z      int
	Heading      int
	RespawnDelay int // seconds (for future use)
	Count        int
}
