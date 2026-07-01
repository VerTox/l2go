package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
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

// handleRequestRestartPoint handles the RequestRestartPoint packet (0x7d): the player
// picked a resurrection point after death. MVP supports "to nearest village" (type 0):
// resolve the nearest town via MapRegion, then hand the game loop a revive-teleport
// (restore HP + Revive + teleport). Without this the death screen's button does nothing.
// (l2go-3xh.4)
func (h *Handler) handleRequestRestartPoint(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt, err := inclient.ParseRequestRestartPoint(payload)
	if err != nil {
		log.Ctx(ctx).Debug().Err(err).Msg("failed to parse RequestRestartPoint")
		return c.Send(outclient.BuildActionFailed())
	}

	session := h.getSession(c)
	if session == nil {
		return c.Send(outclient.BuildActionFailed())
	}
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		return c.Send(outclient.BuildActionFailed())
	}
	logger := log.Ctx(ctx).With().
		Int32("char_id", playerState.CharID).
		Int32("point_type", pkt.RequestedPointType).
		Logger()

	// Only a genuinely dead player may restart (L2J warns/bans on a living one).
	if playerState.Character == nil || playerState.Character.CurrentHP > 0 {
		logger.Warn().Msg("RequestRestartPoint from a living player — ignored")
		return c.Send(outclient.BuildActionFailed())
	}

	// MVP supports only "to nearest town"; other types fall back to town.
	if pkt.RequestedPointType != inclient.RestartPointTown {
		logger.Debug().Msg("unsupported restart point type — defaulting to nearest town")
	}

	respawn, ok := registry.GetMapRegionRegistry().GetRespawnPoint(playerState.Position.X, playerState.Position.Y)
	if !ok {
		logger.Error().Msg("no respawn point resolved — cannot revive")
		return c.Send(outclient.BuildActionFailed())
	}

	h.gameLoopCmd <- gameloop.CmdRevive{
		CharID:  playerState.CharID,
		Dest:    respawn,
		Heading: 0,
	}
	logger.Info().
		Int("respawn_x", respawn.X).
		Int("respawn_y", respawn.Y).
		Msg("revive to nearest town requested")
	return nil
}
