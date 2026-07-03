package gameloop

import (
	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// SetAutoShotSink wires the channel that receives charIDs whose active
// auto-soulshots should be recharged off the loop. The DB consume + charge run on
// the draining goroutine, so the tick never blocks on the database.
func (gl *GameLoop) SetAutoShotSink(sink chan<- int32) {
	gl.autoShotSink = sink
}

// maybeRechargeAutoShots enqueues an off-loop recharge for the attacker when it
// has an active auto-shot and its weapon is not currently charged — i.e. the swing
// just spent the charge (or it was never charged). This mirrors L2J recharging at
// the end of onHitTimer to arm the NEXT swing. Non-blocking: a full sink drops the
// request (the next hit retries). MUST run on the loop goroutine — it reads the
// live Character paperdoll the loop owns.
func (gl *GameLoop) maybeRechargeAutoShots(charID int32) {
	if gl.autoShotSink == nil {
		return
	}
	if !registry.GetAutoShotRegistry().HasAny(charID) {
		return
	}
	player, ok := gl.world.GetPlayer(charID)
	if !ok || player.Character == nil {
		return
	}
	weaponObjID := player.Character.PaperdollObjectIDs[models.SlotRHand]
	if weaponObjID == 0 || registry.GetChargedShotRegistry().IsCharged(weaponObjID, registry.ShotSoulshot) {
		return
	}
	select {
	case gl.autoShotSink <- charID:
	default:
		log.Warn().Int32("char_id", charID).Msg("auto-shot recharge sink full, dropping request")
	}
}
