package gameloop

import (
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// combatTarget is a defender abstraction over an NPC or a player, so the melee
// path (reach, damage, movement) works uniformly. Exactly one of player/npc is set.
type combatTarget struct {
	objectID  int32
	pos       models.Position
	dead      bool
	pDef      int
	evasion   int
	collision float64
	player    *registry.PlayerWorldState // nil if NPC
	npc       *models.NpcInstance        // nil if player
}

func (t *combatTarget) isPlayer() bool { return t.player != nil }

// resolveCombatTarget resolves an object id to a combat defender (player or NPC).
func (gl *GameLoop) resolveCombatTarget(objectID int32) (*combatTarget, bool) {
	if p, ok := gl.world.GetPlayer(objectID); ok {
		if p.Character == nil {
			return nil, false
		}
		st := gl.computePlayerStats(p)
		return &combatTarget{
			objectID:  objectID,
			pos:       p.Position,
			dead:      p.Character.CurrentHP <= 0,
			pDef:      st.PDef,
			evasion:   st.Evasion,
			collision: getCollisionRadiusForPlayer(p.Character.Race, p.Character.Sex),
			player:    p,
		}, true
	}
	if npc, ok := gl.world.GetNPC(objectID); ok {
		pDef, evasion := 10, 20
		var coll float64
		if npc.Template != nil {
			pDef = int(npc.Template.PDef)
			if pDef < 1 {
				pDef = 1
			}
			evasion = int(npc.Template.PDef / 10)
			coll = npc.Template.CollisionRadius
		}
		return &combatTarget{
			objectID: objectID, pos: npc.Position, dead: npc.IsDead,
			pDef: pDef, evasion: evasion, collision: coll, npc: npc,
		}, true
	}
	return nil, false
}

// meleeReachTo is meleeReach generalized to any defender.
func (gl *GameLoop) meleeReachTo(player *registry.PlayerWorldState, t *combatTarget) int {
	var pc float64
	if player.Character != nil {
		pc = getCollisionRadiusForPlayer(player.Character.Race, player.Character.Sex)
	}
	return attackReach(playerPhysicalAttackRange(player), pc, t.collision)
}

// startMoveToTargetPos is startMoveToTarget generalized to a target position/id.
func (gl *GameLoop) startMoveToTargetPos(player *registry.PlayerWorldState, targetObjID int32, targetPos models.Position, reach int) {
	dest := stopPointWithinReach(player.Position, targetPos, reach)
	if dest == player.Position {
		return
	}
	player.IsMoving = true
	player.MoveStartPos = player.Position
	player.MoveDestination = dest
	player.MoveStarted = time.Now()
	gl.approachTargetPos(player.AccountName, player, targetObjID, targetPos, reach)
}

// approachTargetPos is approachTarget generalized to a target position/id.
func (gl *GameLoop) approachTargetPos(accountName string, player *registry.PlayerWorldState, targetObjID int32, targetPos models.Position, reach int) {
	conn := gl.connections.GetConnection(accountName)
	if conn == nil {
		return
	}
	_ = conn.Send(outclient.BuildMoveToPawn(
		player.CharID, targetObjID, int32(reach),
		player.Position.X, player.Position.Y, player.Position.Z,
		targetPos.X, targetPos.Y, targetPos.Z,
	))
}
