package gameloop

import (
	"math"

	"github.com/VerTox/l2go/internal/gameserver/models"
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
