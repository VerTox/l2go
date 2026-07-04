package gameloop

import (
	"math"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// Intention is the player's current AI intention (L2J AI_INTENTION_*).
type Intention int

const (
	IntentionIdle Intention = iota
	IntentionMoveTo
	IntentionAttack
	IntentionInteract
	IntentionCast   // scaffold — skill system not implemented yet
	IntentionFollow // scaffold — follow not implemented yet
)

// PlayerAIState holds a player's current intention and its target.
type PlayerAIState struct {
	Intention      Intention
	TargetObjectID int32           // for Attack / Interact / Cast
	MoveDest       models.Position // for MoveTo
}

// setIntention records the player's intention and target, replacing any previous one.
func (gl *GameLoop) setIntention(charID int32, intention Intention, targetObjectID int32) {
	st, ok := gl.aiState[charID]
	if !ok {
		st = &PlayerAIState{}
		gl.aiState[charID] = st
	}
	st.Intention = intention
	st.TargetObjectID = targetObjectID
}

// clearIntention resets the player to idle.
func (gl *GameLoop) clearIntention(charID int32) {
	if st, ok := gl.aiState[charID]; ok {
		st.Intention = IntentionIdle
		st.TargetObjectID = 0
	}
}

// startMoveToTarget begins SERVER-SIDE movement of the player toward npc, stopping
// within `reach`. It sets the movement fields the tick interpolation (phase 1) reads,
// and sends a MoveToPawn so the client walks the same path. No-op if already in reach.
func (gl *GameLoop) startMoveToTarget(player *registry.PlayerWorldState, npc *models.NpcInstance, reach int) {
	gl.startMoveToTargetPos(player, npc.ObjectID, npc.Position, reach)
}

// onMovementArrived is called by the tick when a player's server-side movement
// completes. It dispatches on the player's current intention.
func (gl *GameLoop) onMovementArrived(charID int32) {
	st, ok := gl.aiState[charID]
	if !ok {
		return
	}
	switch st.Intention {
	case IntentionAttack:
		// No-op: the combat heartbeat (NextAttackEvent) drives the swing chain.
		// beginAttackSwing from handleAttackRequest starts the chain; the heartbeat
		// re-schedules itself (in-range: next swing; out-of-range: 400 ms retry).
		// Calling beginAttackSwing here would create a second parallel chain.
	case IntentionInteract:
		// No-op: the interact heartbeat (InteractApproachEvent) opens the dialogue on
		// arrival, mirroring the attack no-op above.
	default:
		// MoveTo / Idle / scaffolded intentions: nothing to do on arrival
	}
}

// beginAttackSwing schedules an immediate attack swing (no approach delay); range
// is re-checked inside NextAttackEvent.
func (gl *GameLoop) beginAttackSwing(charID int32, targetObjectID int32) {
	cs, ok := gl.combatState[charID]
	if !ok || !cs.IsAutoAttacking || cs.TargetObjectID != targetObjectID {
		return
	}
	gl.events.Schedule(&NextAttackEvent{
		At:             time.Now(),
		AttackerCharID: charID,
		TargetObjectID: targetObjectID,
	})
}

// stopPointWithinReach returns the point on the line from target toward `from`
// at distance `reach` from target — i.e. where the mover should stop to be just
// within reach of target. If `from` is already within reach, returns `from`.
func stopPointWithinReach(from, target models.Position, reach int) models.Position {
	dx := float64(from.X - target.X)
	dy := float64(from.Y - target.Y)
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist <= float64(reach) || dist == 0 {
		return from
	}
	ratio := float64(reach) / dist
	return models.Position{
		X: target.X + int(dx*ratio),
		Y: target.Y + int(dy*ratio),
		Z: from.Z,
	}
}
