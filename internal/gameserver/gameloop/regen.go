package gameloop

import (
	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
)

// regenPlayers restores HP/MP/CP to every living player once per regen tick and
// pushes the updated bars to the client (and anyone targeting the player). Runs on
// the loop goroutine — the sole writer of Character vitals — so no locking is
// needed. Dead players (HP <= 0) and players already at full on all three vitals
// are skipped (no packet).
//
// The regen amounts are a level-only approximation of the retail per-level table
// (models.*RegenPerTick); CON/MEN, sitting/standing/running and combat modifiers
// are a follow-up (l2go-y93). This exists so MP actually replenishes, making
// MP-consuming skills viable (epic l2go-z36).
func (gl *GameLoop) regenPlayers() {
	for charID, player := range gl.world.GetAllPlayers() {
		char := player.Character
		if char == nil || char.CurrentHP <= 0 {
			continue // missing or dead — no regen
		}

		newHP := clampFloat(char.CurrentHP+models.HpRegenPerTick(char.Level), float64(char.MaxHP))
		newMP := clampFloat(char.CurrentMP+models.MpRegenPerTick(char.Level), float64(char.MaxMP))
		newCP := clampInt(char.CurrentCP+int(models.CpRegenPerTick(char.Level)), char.MaxCP)

		if newHP == char.CurrentHP && newMP == char.CurrentMP && newCP == char.CurrentCP {
			continue // already full — nothing to update
		}
		char.CurrentHP, char.CurrentMP, char.CurrentCP = newHP, newMP, newCP

		su := outclient.BuildStatusUpdate(charID, []outclient.StatusAttribute{
			{ID: outclient.StatusCurHP, Value: int32(newHP)},
			{ID: outclient.StatusCurMP, Value: int32(newMP)},
			{ID: outclient.StatusCurCP, Value: int32(newCP)},
		})
		if conn := gl.connections.GetConnection(player.AccountName); conn != nil {
			_ = conn.Send(su)
		}
		gl.broadcastToTargeters(charID, su)
	}
}
