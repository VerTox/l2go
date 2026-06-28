package outclient

import (
	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// BuildNpcInfo builds the NpcInfo packet (0x0C) for sending NPC data to a client.
// Follows Java L2J AbstractNpcInfo structure.
func BuildNpcInfo(npc *models.NpcInstance) []byte {
	t := npc.Template
	w := l2pkt.NewWriter()

	w.WriteC(0x0C) // opcode

	w.WriteD(npc.ObjectID)                    // objectId
	w.WriteD(npc.TemplateID + 1_000_000)      // npcTypeId = npcId + 1000000
	w.WriteD(boolToD(t.Attackable))           // isAttackable
	w.WriteD(int32(npc.Position.X))           // x
	w.WriteD(int32(npc.Position.Y))           // y
	w.WriteD(int32(npc.Position.Z))           // z
	w.WriteD(npc.Heading)                     // heading
	w.WriteD(0)                               // _unk1
	w.WriteD(int32(t.MAtkSpd))                // MAtkSpd
	w.WriteD(int32(t.PAtkSpd))                // PAtkSpd
	w.WriteD(int32(t.RunSpd))                 // runSpeed
	w.WriteD(int32(t.WalkSpd))                // walkSpeed
	w.WriteD(int32(t.RunSpd))                 // swimRunSpd (reuse run)
	w.WriteD(int32(t.WalkSpd))                // swimWalkSpd (reuse walk)
	w.WriteD(0)                               // flyRunSpd
	w.WriteD(0)                               // flyWalkSpd
	w.WriteD(0)                               // flyRunSpd (duplicate)
	w.WriteD(0)                               // flyWalkSpd (duplicate)

	// Movement and attack speed multipliers
	moveMultiplier := 1.0
	if t.RunSpd > 0 {
		moveMultiplier = float64(t.RunSpd) / 120.0 // baseRunSpd typically 120
	}
	w.WriteF(moveMultiplier) // moveSpeedMultiplier
	w.WriteF(1.0)            // attackSpeedMultiplier

	w.WriteF(t.CollisionRadius) // collisionRadius
	w.WriteF(t.CollisionHeight) // collisionHeight
	w.WriteD(t.RHand)           // rhand weapon
	w.WriteD(t.Chest)           // chest armor
	w.WriteD(t.LHand)           // lhand weapon/shield

	w.WriteC(1) // nameAbove (1 = show name above NPC)
	w.WriteC(boolToC(npc.IsRunning))
	w.WriteC(0) // isInCombat
	w.WriteC(boolToC(npc.IsDead))
	w.WriteC(0) // isSummoned (0 = normal NPC)

	w.WriteD(-1)         // npcStringId for name (-1 = use string)
	w.WriteS(t.Name)     // name
	w.WriteD(-1)         // npcStringId for title (-1 = use string)
	w.WriteS(t.Title)    // title
	w.WriteD(0)          // titleColor
	w.WriteD(0)          // pvpFlag
	w.WriteD(0)          // karma
	w.WriteD(0)          // abnormalEffect
	w.WriteD(0)          // clanId
	w.WriteD(0)          // clanCrestId
	w.WriteD(0)          // allyId
	w.WriteD(0)          // allyCrestId
	w.WriteC(0)          // isFlying (0 = ground)
	w.WriteC(0)          // teamId
	w.WriteF(t.CollisionRadius) // collisionRadius (duplicate for riding)
	w.WriteF(t.CollisionHeight) // collisionHeight (duplicate for riding)
	w.WriteD(0)          // enchantEffect
	w.WriteD(0)          // isFlying2
	w.WriteD(0)          // _unk4
	w.WriteD(0)          // colorEffect
	w.WriteC(boolToC(t.Targetable))
	w.WriteC(boolToC(t.ShowName))
	w.WriteD(0)          // specialAbnormalEffect
	w.WriteD(0)          // displayEffect

	return w.Bytes()
}

// boolToD converts a bool to int32 (0 or 1).
func boolToD(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// boolToC converts a bool to byte (0 or 1).
func boolToC(b bool) byte {
	if b {
		return 1
	}
	return 0
}
