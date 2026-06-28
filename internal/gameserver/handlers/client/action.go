package client

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

// handleRequestActionUse processes action use requests (opcode 0x56)
// Based on Java L2J: Action ID 1 = Walk/Run toggle
func (h *Handler) handleRequestActionUse(ctx context.Context, c *client.ClientConn, payload []byte) error {
	// Parse action use packet
	actionPacket, err := inclient.ParseRequestActionUse(payload)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to parse RequestActionUse packet")
		return fmt.Errorf("failed to parse action packet: %w", err)
	}

	// Validate packet data
	if err := actionPacket.Validate(); err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("action packet validation failed")
		return fmt.Errorf("invalid action packet: %w", err)
	}

	// Get session to access account name
	session := h.getSession(c)
	if session == nil {
		log.Ctx(ctx).Error().Msg("no session found for action request")
		return nil
	}

	// Get player from world registry
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		log.Ctx(ctx).Error().Str("account", session.AccountName).Msg("player not found in world for action")
		return nil
	}

	logger := log.Ctx(ctx).With().
		Str("account", session.AccountName).
		Int32("char_id", playerState.CharID).
		Int32("action_id", actionPacket.ActionID).
		Bool("ctrl", actionPacket.CtrlPressed).
		Bool("shift", actionPacket.ShiftPressed).
		Logger()

	logger.Debug().Msg("processing action use request")

	// Handle specific actions
	switch actionPacket.ActionID {
	case 1: // Walk/Run toggle (most important action)
		return h.handleWalkRunToggle(ctx, c, playerState, logger)
	
	case 0: // Sit/Stand toggle
		logger.Debug().Msg("sit/stand toggle not implemented yet")
		// TODO: Implement sit/stand functionality
		return nil
		
	default:
		logger.Debug().Msg("unimplemented action ID")
		// For now, just log unimplemented actions without failing
		return nil
	}
}

// handleWalkRunToggle handles the Walk/Run toggle action (Action ID 1)
func (h *Handler) handleWalkRunToggle(ctx context.Context, c *client.ClientConn, playerState *registry.PlayerWorldState, logger zerolog.Logger) error {
	// Toggle the run/walk state
	newRunning := !playerState.IsRunning
	
	logger.Info().
		Bool("was_running", playerState.IsRunning).
		Bool("now_running", newRunning).
		Msg("toggling walk/run state")

	// Update player state in world registry
	if err := h.world.UpdatePlayerRunWalkState(ctx, playerState.CharID, newRunning); err != nil {
		logger.Error().Err(err).Msg("failed to update run/walk state")
		return fmt.Errorf("failed to update run/walk state: %w", err)
	}

	// Send ChangeMoveType packet to inform client and nearby players
	changeMoveType := outclient.NewChangeMoveType(playerState.CharID, newRunning)
	if err := c.Send(changeMoveType.GetData()); err != nil {
		logger.Error().Err(err).Msg("failed to send ChangeMoveType to self")
		return fmt.Errorf("failed to send move type change: %w", err)
	}

	// Broadcast to visible players (excluding self)
	if err := h.broadcastChangeMoveTypeToVisiblePlayers(ctx, playerState.CharID, newRunning, c); err != nil {
		logger.Warn().Err(err).Msg("failed to broadcast ChangeMoveType")
		// Don't fail the action for broadcast errors
	}

	logger.Info().
		Bool("is_running", newRunning).
		Msg("walk/run state changed successfully")

	return nil
}

// broadcastChangeMoveTypeToVisiblePlayers broadcasts movement type change to visible players
func (h *Handler) broadcastChangeMoveTypeToVisiblePlayers(ctx context.Context, charID int32, isRunning bool, excludeClient *client.ClientConn) error {
	// Get visible players
	visiblePlayers, err := h.movementUseCase.GetVisiblePlayers(ctx, charID)
	if err != nil {
		return fmt.Errorf("failed to get visible players: %w", err)
	}

	if len(visiblePlayers) == 0 {
		return nil // No players to notify
	}

	logger := log.Ctx(ctx).With().
		Int32("char_id", charID).
		Bool("is_running", isRunning).
		Int("visible_count", len(visiblePlayers)).
		Logger()

	// Create ChangeMoveType packet for broadcasting
	changeMoveType := outclient.NewChangeMoveType(charID, isRunning)
	packetData := changeMoveType.GetData()
	broadcastCount := 0

	// Broadcast to all visible players
	for _, visiblePlayer := range visiblePlayers {
		// Find client connection for this visible player by account name
		visibleClient := h.findClientByAccount(visiblePlayer.AccountName)
		
		// Don't send to the player who initiated the change (they already got the packet)
		if visibleClient != nil && visibleClient != excludeClient {
			if err := visibleClient.Send(packetData); err != nil {
				logger.Warn().Err(err).
					Str("target_account", visiblePlayer.AccountName).
					Msg("failed to broadcast ChangeMoveType to player")
			} else {
				broadcastCount++
				logger.Debug().
					Str("target_account", visiblePlayer.AccountName).
					Msg("ChangeMoveType broadcasted to player")
			}
		}
	}

	logger.Debug().
		Int("broadcasts_sent", broadcastCount).
		Msg("ChangeMoveType broadcast completed")

	return nil
}