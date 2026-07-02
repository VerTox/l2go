package gameloop

import (
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

// handleRestoreStats applies a HP/MP/CP restore to a live player (clamped to maxima)
// and pushes the updated bars to the client and anyone targeting the player.
//
// INTERIM (l2go-diu): this is the landing point for potion effects until a real
// skill engine exists. It only restores vitals; it never revives a dead player.
func (gl *GameLoop) handleRestoreStats(cmd CmdRestoreStats) {
	player, exists := gl.world.GetPlayer(cmd.CharID)
	if !exists || player.Character == nil {
		return
	}
	char := player.Character
	if char.CurrentHP <= 0 {
		return // dead players are not healed by potions
	}

	if cmd.HP > 0 {
		char.CurrentHP = clampFloat(char.CurrentHP+float64(cmd.HP), float64(char.MaxHP))
	}
	if cmd.MP > 0 {
		char.CurrentMP = clampFloat(char.CurrentMP+float64(cmd.MP), float64(char.MaxMP))
	}
	if cmd.CP > 0 {
		char.CurrentCP = clampInt(char.CurrentCP+int(cmd.CP), char.MaxCP)
	}

	// HP/MP/CP bars: sent to the player and to anyone currently targeting it.
	su := outclient.BuildStatusUpdate(cmd.CharID, []outclient.StatusAttribute{
		{ID: outclient.StatusCurHP, Value: int32(char.CurrentHP)},
		{ID: outclient.StatusMaxHP, Value: int32(char.MaxHP)},
		{ID: outclient.StatusCurMP, Value: int32(char.CurrentMP)},
		{ID: outclient.StatusMaxMP, Value: int32(char.MaxMP)},
		{ID: outclient.StatusCurCP, Value: int32(char.CurrentCP)},
		{ID: outclient.StatusMaxCP, Value: int32(char.MaxCP)},
	})
	if conn := gl.connections.GetConnection(player.AccountName); conn != nil {
		_ = conn.Send(su)
		// UserInfo keeps the player's own frames (HP/MP/CP fields) authoritative.
		_ = conn.Send(gl.buildUserInfoForPlayer(player))
	}
	gl.broadcastToTargeters(cmd.CharID, su)
}

// statRestorer adapts the game loop's command channel to usecase.StatRestorer so
// item handlers can request a restore without depending on the loop internals.
type statRestorer struct {
	ch chan<- Command
}

func (s statRestorer) RestoreStats(charID, hp, mp, cp int32) {
	s.ch <- CmdRestoreStats{CharID: charID, HP: hp, MP: mp, CP: cp}
}

// StatRestorer returns a usecase.StatRestorer backed by this loop's command channel.
func (gl *GameLoop) StatRestorer() usecase.StatRestorer {
	return statRestorer{ch: gl.commands}
}

// ensure the adapter satisfies the usecase contract at compile time.
var _ usecase.StatRestorer = statRestorer{}

// SkillEffectSource returns a skill-effect lookup for the interim potion handler.
// It is a thin re-export so callers wire it without importing registry directly.
var _ usecase.SkillEffectSource = (*registry.SkillEffectRegistry)(nil)

func clampFloat(v, max float64) float64 {
	if v > max {
		return max
	}
	return v
}

func clampInt(v, max int) int {
	if v > max {
		return max
	}
	return v
}
