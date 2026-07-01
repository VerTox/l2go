package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

// handleAppearing handles the Appearing packet (0x3a): the client reports it has
// finished loading the area after a teleport. If a teleport was in flight, we
// re-establish visibility at the destination (the client unloaded the old area on
// TeleportToLocation), refresh UserInfo, and clear the teleporting flag — the second
// half of the L2J teleToLocation/onTeleported flow. (l2go-3xh.2)
func (h *Handler) handleAppearing(ctx context.Context, c *client.ClientConn, _ []byte) error {
	session := h.getSession(c)
	if session == nil {
		return nil
	}
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		return nil
	}
	logger := log.Ctx(ctx).With().Int32("char_id", playerState.CharID).Logger()

	if playerState.IsTeleporting {
		// Reset the known-object set (old area is gone client-side) and rebuild visibility
		// at the new position.
		playerState.KnownNPCs = make(map[int32]bool)
		h.establishPlayerVisibility(ctx, c, playerState)
		playerState.IsTeleporting = false

		if err := c.Send(h.buildUserInfoPacket(playerState.Character)); err != nil {
			logger.Warn().Err(err).Msg("failed to send UserInfo after teleport arrival")
		}

		// Re-activate regions around the destination.
		h.gameLoopCmd <- gameloop.CmdPlayerEnteredWorld{
			CharID:      playerState.CharID,
			AccountName: session.AccountName,
			Position:    playerState.Position,
		}
		logger.Info().Msg("teleport arrival processed (Appearing)")
		return nil
	}

	// Non-teleport Appearing: refresh UserInfo, matching L2J which always answers here.
	return c.Send(h.buildUserInfoPacket(playerState.Character))
}
