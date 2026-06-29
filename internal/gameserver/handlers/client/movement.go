package client

import (
	"context"
	"fmt"
	"math"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

// handleMoveBackwardToLocation handles client movement requests
func (h *Handler) handleMoveBackwardToLocation(ctx context.Context, c *client.ClientConn, payload []byte) error {
	logger := log.Ctx(ctx).With().Str("handler", "MoveBackwardToLocation").Logger()

	// Get session to find character ID
	session := h.getSession(c)
	if session == nil {
		return fmt.Errorf("no session found for client")
	}

	// Get character from world registry
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		return fmt.Errorf("player not found in world: %s", session.AccountName)
	}

	// Parse movement packet
	movePacket, err := inclient.ParseMoveBackwardToLocation(payload)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse MoveBackwardToLocation packet")
		return fmt.Errorf("failed to parse movement packet: %w", err)
	}

	logger.Debug().
		Int32("target_x", movePacket.TargetX).
		Int32("target_y", movePacket.TargetY).
		Int32("target_z", movePacket.TargetZ).
		Int32("client_origin_x", movePacket.OriginX).
		Int32("client_origin_y", movePacket.OriginY).
		Int32("client_origin_z", movePacket.OriginZ).
		Int32("server_pos_x", int32(playerState.Position.X)).
		Int32("server_pos_y", int32(playerState.Position.Y)).
		Int32("server_pos_z", int32(playerState.Position.Z)).
		Int32("move_type", movePacket.MoveMovement).
		Msg("movement request received")

	// Validate packet data
	if err := movePacket.Validate(); err != nil {
		logger.Warn().Err(err).Msg("movement packet validation failed")
		return fmt.Errorf("invalid movement packet: %w", err)
	}

	// Create destination position
	destination := models.Position{
		X: int(movePacket.TargetX),
		Y: int(movePacket.TargetY),
		Z: int(movePacket.TargetZ),
	}

	// CRITICAL FIX: Use client's origin position as more accurate starting point
	// The client knows exactly where it thinks it is
	clientOrigin := models.Position{
		X: int(movePacket.OriginX),
		Y: int(movePacket.OriginY),
		Z: int(movePacket.OriginZ),
	}

	// Update server position to match client's origin for accuracy
	if err := h.movementUseCase.UpdatePosition(ctx, playerState.CharID, clientOrigin, 0); err != nil {
		logger.Warn().Err(err).Msg("failed to sync position with client origin")
		// Continue with movement anyway
	}

	// CRITICAL FIX: Use player's persistent run/walk state, not distance
	// Based on L2J analysis: players have isRunning boolean that persists
	isRunning := playerState.IsRunning // Get from player's persistent state

	logger.Debug().
		Bool("is_running", isRunning).
		Msg("using player run/walk state for movement")

	// Start movement via use case
	result, err := h.movementUseCase.StartMovement(ctx, playerState.CharID, destination, isRunning)
	if err != nil {
		logger.Error().Err(err).Msg("failed to start movement")
		return fmt.Errorf("movement failed: %w", err)
	}

	// Check if movement was successful
	if !result.Success {
		logger.Warn().
			Interface("validation_error", result.ValidationError).
			Msg("movement validation failed")

		// Send position correction if needed
		if result.ValidationError != nil {
			return h.sendPositionCorrection(ctx, c, playerState.CharID, result.StartPosition)
		}
		return nil
	}

	// CRITICAL: Send MoveToLocation confirmation to the client first
	// Without this, client will ignore subsequent movement requests
	moveConfirmation := outclient.NewMoveToLocation(
		playerState.CharID,
		int32(destination.X), int32(destination.Y), int32(destination.Z),
		int32(result.StartPosition.X), int32(result.StartPosition.Y), int32(result.StartPosition.Z),
	)

	if err := c.Send(moveConfirmation.GetData()); err != nil {
		logger.Error().Err(err).Msg("failed to send MoveToLocation confirmation to client")
		return fmt.Errorf("failed to confirm movement: %w", err)
	}

	logger.Debug().Msg("MoveToLocation confirmation sent to client")

	// Broadcast movement to visible players (excluding the moving player)
	if err := h.broadcastMovementToVisiblePlayers(ctx, result, c); err != nil {
		logger.Warn().Err(err).Msg("failed to broadcast movement")
		// Don't fail the movement for broadcast errors
	}

	// REMOVED: updatePlayerVisibility - CharInfo should only be sent on status changes, not movement

	logger.Info().
		Int32("char_id", playerState.CharID).
		Float64("distance", h.calculateDistance(result.StartPosition, result.TargetPosition)).
		Int("visible_players", len(result.VisiblePlayers)).
		Msg("movement processed successfully")

	return nil
}

// handleValidatePosition handles client position validation requests
func (h *Handler) handleValidatePosition(ctx context.Context, c *client.ClientConn, payload []byte) error {
	logger := log.Ctx(ctx).With().Str("handler", "ValidatePosition").Logger()

	// Get session to find character ID
	session := h.getSession(c)
	if session == nil {
		return fmt.Errorf("no session found for client")
	}

	// Get character from world registry
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		return fmt.Errorf("player not found in world: %s", session.AccountName)
	}

	// Parse validation packet
	validatePacket, err := inclient.ParseValidatePosition(payload)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse ValidatePosition packet")
		return fmt.Errorf("failed to parse validation packet: %w", err)
	}

	// Validate packet data
	if err := validatePacket.Validate(); err != nil {
		logger.Warn().Err(err).Msg("position validation packet validation failed")
		return fmt.Errorf("invalid validation packet: %w", err)
	}

	logger.Debug().
		Int32("client_x", validatePacket.X).
		Int32("client_y", validatePacket.Y).
		Int32("client_z", validatePacket.Z).
		Int32("heading", validatePacket.Heading).
		Msg("position validation request")

	// Create client position
	clientPos := models.Position{
		X: int(validatePacket.X),
		Y: int(validatePacket.Y),
		Z: int(validatePacket.Z),
	}

	// While the server tick is the position authority during movement, skip
	// drift correction entirely — the interpolation uses a hardcoded speed so
	// any real speed delta would accumulate and trigger rubber-band.
	// Only validate + sync position when the player is standing still.
	if !playerState.IsMoving {
		// Validate client position against server expectation
		correctionResult, err := h.movementUseCase.ValidateClientPosition(
			ctx, playerState.CharID, clientPos, validatePacket.Heading,
		)
		if err != nil {
			logger.Error().Err(err).Msg("failed to validate client position")
			return fmt.Errorf("position validation failed: %w", err)
		}

		// Send position correction if needed
		if correctionResult.NeedsCorrection {
			logger.Warn().
				Float64("deviation", correctionResult.Deviation).
				Int("expected_x", correctionResult.ExpectedPos.X).
				Int("expected_y", correctionResult.ExpectedPos.Y).
				Int("expected_z", correctionResult.ExpectedPos.Z).
				Int("client_x", correctionResult.ClientPos.X).
				Int("client_y", correctionResult.ClientPos.Y).
				Int("client_z", correctionResult.ClientPos.Z).
				Msg("position correction required")

			return h.sendPositionCorrection(ctx, c, playerState.CharID, correctionResult.ExpectedPos)
		}

		// Sync authoritative client position while standing still.
		if err := h.movementUseCase.UpdatePosition(ctx, playerState.CharID, clientPos, validatePacket.Heading); err != nil {
			logger.Warn().Err(err).Msg("failed to update position")
			// Don't fail validation for position update errors
		}

		logger.Debug().
			Float64("deviation", correctionResult.Deviation).
			Msg("position validation passed")
	} else {
		logger.Debug().Msg("skip drift validation while moving — tick is position authority")
	}

	// Notify game loop about position change (for active region tracking)
	h.gameLoopCmd <- gameloop.CmdPlayerMoved{
		CharID:   playerState.CharID,
		Position: clientPos,
	}

	// Update NPC visibility as player moves
	h.updateNPCVisibility(ctx, c, playerState)

	return nil
}

// handleCannotMoveAnymore handles client notification of movement completion/stop
func (h *Handler) handleCannotMoveAnymore(ctx context.Context, c *client.ClientConn, payload []byte) error {
	logger := log.Ctx(ctx).With().Str("handler", "CannotMoveAnymore").Logger()

	// Get session to find character ID
	session := h.getSession(c)
	if session == nil {
		return fmt.Errorf("no session found for client")
	}

	// Get character from world registry
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		return fmt.Errorf("player not found in world: %s", session.AccountName)
	}

	// Parse movement stop packet
	stopPacket, err := inclient.ParseCannotMoveAnymore(payload)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse CannotMoveAnymore packet")
		return fmt.Errorf("failed to parse movement stop packet: %w", err)
	}

	// Validate packet data
	if err := stopPacket.Validate(); err != nil {
		logger.Warn().Err(err).Msg("movement stop packet validation failed")
		return fmt.Errorf("invalid movement stop packet: %w", err)
	}

	logger.Debug().
		Int32("final_x", stopPacket.X).
		Int32("final_y", stopPacket.Y).
		Int32("final_z", stopPacket.Z).
		Int32("heading", stopPacket.Heading).
		Msg("movement stop notification received")

	// Create final position
	finalPosition := models.Position{
		X: int(stopPacket.X),
		Y: int(stopPacket.Y),
		Z: int(stopPacket.Z),
	}

	// Stop movement via use case and update final position
	if err := h.movementUseCase.StopMovement(ctx, playerState.CharID); err != nil {
		logger.Error().Err(err).Msg("failed to stop movement")
		return fmt.Errorf("movement stop failed: %w", err)
	}

	// Update position to final location
	if err := h.movementUseCase.UpdatePosition(ctx, playerState.CharID, finalPosition, stopPacket.Heading); err != nil {
		logger.Warn().Err(err).Msg("failed to update final position")
		// Don't fail the stop for position update errors
	}

	logger.Info().
		Int32("char_id", playerState.CharID).
		Int("final_x", finalPosition.X).
		Int("final_y", finalPosition.Y).
		Int("final_z", finalPosition.Z).
		Msg("movement stopped successfully")

	return nil
}

// Helper methods

// broadcastMovementToVisiblePlayers broadcasts movement to all visible players
func (h *Handler) broadcastMovementToVisiblePlayers(ctx context.Context, result *usecase.MovementResult, excludeClient *client.ClientConn) error {
	if len(result.VisiblePlayers) == 0 {
		return nil // No players to notify
	}

	logger := log.Ctx(ctx).With().
		Int32("moving_char", result.CharacterID).
		Int("visible_count", len(result.VisiblePlayers)).
		Logger()

	// Create MoveToLocation packet for broadcasting
	movePacket := outclient.NewMoveToLocation(
		result.CharacterID,
		int32(result.TargetPosition.X), int32(result.TargetPosition.Y), int32(result.TargetPosition.Z),
		int32(result.StartPosition.X), int32(result.StartPosition.Y), int32(result.StartPosition.Z),
	)

	packetData := movePacket.GetData()
	broadcastCount := 0

	// Broadcast to all visible players
	for _, visiblePlayer := range result.VisiblePlayers {
		// Find client connection for this visible player by account name
		visibleClient := h.findClientByAccount(visiblePlayer.AccountName)

		// Don't send to the moving player (they already got confirmation)
		if visibleClient != nil && visibleClient != excludeClient {
			if err := visibleClient.Send(packetData); err != nil {
				logger.Warn().Err(err).
					Str("target_account", visiblePlayer.AccountName).
					Msg("failed to broadcast movement to player")
			} else {
				broadcastCount++
				logger.Debug().
					Str("target_account", visiblePlayer.AccountName).
					Msg("movement broadcasted to player")
			}
		}
	}

	logger.Debug().
		Int("broadcasts_sent", broadcastCount).
		Msg("movement broadcast completed")

	return nil
}

// findClientByAccount finds client connection by account name
// TODO: This is O(n) lookup - in production, use a hash map for O(1) lookup
func (h *Handler) findClientByAccount(accountName string) *client.ClientConn {
	// Iterate through all active sessions to find matching account
	for clientConn, session := range h.sessions {
		if session != nil && session.AccountName == accountName {
			return clientConn
		}
	}
	return nil // Client not found or disconnected
}

// sendPositionCorrection sends a position correction packet to the client
func (h *Handler) sendPositionCorrection(ctx context.Context, c *client.ClientConn, charID int32, correctPos models.Position) error {
	logger := log.Ctx(ctx).With().
		Int32("char_id", charID).
		Int("correct_x", correctPos.X).
		Int("correct_y", correctPos.Y).
		Int("correct_z", correctPos.Z).
		Logger()

	// Create position correction packet
	correctionPacket := outclient.NewValidatePositionServer(
		charID,
		int32(correctPos.X), int32(correctPos.Y), int32(correctPos.Z),
		0, // heading - TODO: get actual heading
	)

	packetData := correctionPacket.Build()

	if err := c.Send(packetData); err != nil {
		logger.Error().Err(err).Msg("failed to send position correction")
		return fmt.Errorf("failed to send position correction: %w", err)
	}

	logger.Info().Msg("position correction sent to client")
	return nil
}

// calculateDistance calculates 2D distance between positions
func (h *Handler) calculateDistance(pos1, pos2 models.Position) float64 {
	dx := float64(pos2.X - pos1.X)
	dy := float64(pos2.Y - pos1.Y)
	return math.Sqrt(dx*dx + dy*dy) // Return actual distance, not squared
}

// updatePlayerVisibility handles dynamic player visibility updates during movement
// Players appear/disappear as they enter/leave visibility range
func (h *Handler) updatePlayerVisibility(ctx context.Context, movingCharID int32, movingClient *client.ClientConn) {
	logger := log.Ctx(ctx).With().
		Int32("moving_char_id", movingCharID).
		Logger()

	// Get moving player's current state
	movingPlayer, exists := h.world.GetPlayer(movingCharID)
	if !exists {
		logger.Debug().Msg("moving player not found in world registry")
		return
	}

	// Get all players in visibility range from new position
	const VISIBILITY_RADIUS = 1500
	nearbyPlayers := h.world.GetPlayersInRange(movingPlayer.Position, VISIBILITY_RADIUS)

	// Track visibility changes
	var newVisiblePlayers []string   // account names of newly visible players
	var newVisibleToPlayers []string // account names of players who can now see the moving player

	logger.Debug().
		Int("nearby_players_found", len(nearbyPlayers)).
		Msg("checking visibility updates")

	// Process each nearby player
	for _, nearby := range nearbyPlayers {
		if nearby.CharID == movingCharID {
			continue // Skip self
		}

		nearbyConn := h.getConnectionByAccount(nearby.Character.AccountName)
		if nearbyConn == nil {
			continue // Player not connected
		}

		// Check if this nearby player is newly visible to the moving player
		// For now, we'll show all nearby players (in production, track previous visibility state)
		if h.shouldShowPlayerToPlayer(movingPlayer, nearby) {
			// Send CharInfo of nearby player to moving player
			if err := h.sendPlayerSpawnToClient(ctx, movingClient, nearby.Character); err != nil {
				logger.Warn().Err(err).
					Str("nearby_account", nearby.Character.AccountName).
					Msg("failed to show nearby player to moving player")
			} else {
				newVisiblePlayers = append(newVisiblePlayers, nearby.Character.AccountName)
			}
		}

		// Check if moving player is newly visible to this nearby player
		if h.shouldShowPlayerToPlayer(nearby, movingPlayer) {
			// Send CharInfo of moving player to nearby player
			if err := h.sendPlayerSpawnToClient(ctx, nearbyConn, movingPlayer.Character); err != nil {
				logger.Warn().Err(err).
					Str("nearby_account", nearby.Character.AccountName).
					Msg("failed to show moving player to nearby player")
			} else {
				newVisibleToPlayers = append(newVisibleToPlayers, nearby.Character.AccountName)
			}
		}
	}

	if len(newVisiblePlayers) > 0 || len(newVisibleToPlayers) > 0 {
		logger.Info().
			Int("newly_visible_to_moving", len(newVisiblePlayers)).
			Int("newly_visible_to_others", len(newVisibleToPlayers)).
			Msg("visibility updates completed")
	}
}

// shouldShowPlayerToPlayer determines if playerA should see playerB
// In a full implementation, this would check previous visibility state
// For now, we'll use a simple distance-based approach
func (h *Handler) shouldShowPlayerToPlayer(playerA, playerB *registry.PlayerWorldState) bool {
	// Simple distance check (in production, track previous visibility)
	distance := h.calculateDistance(playerA.Position, playerB.Position)
	return distance <= 1500.0 // Within visibility radius
}

// updateNPCVisibility sends NpcInfo for newly visible NPCs and DeleteObject
// for NPCs that left the visibility range. Called during position updates.
func (h *Handler) updateNPCVisibility(ctx context.Context, c *client.ClientConn, playerState *registry.PlayerWorldState) {
	const NPC_VISIBILITY_RADIUS = 2500

	// Get NPCs currently in range
	nearbyNPCs := h.world.GetNPCsInRange(playerState.Position, NPC_VISIBILITY_RADIUS)

	// Build a set of currently visible NPC objectIDs
	currentVisible := make(map[int32]bool, len(nearbyNPCs))
	for _, npc := range nearbyNPCs {
		currentVisible[npc.ObjectID] = true
	}

	// Send NpcInfo for NPCs entering visibility range
	newCount := 0
	for _, npc := range nearbyNPCs {
		if !playerState.KnownNPCs[npc.ObjectID] {
			npcInfoData := outclient.BuildNpcInfo(npc)
			if err := c.Send(npcInfoData); err != nil {
				log.Ctx(ctx).Warn().Err(err).
					Int32("npc_obj_id", npc.ObjectID).
					Msg("failed to send NpcInfo for newly visible NPC")
			} else {
				newCount++
			}
			playerState.KnownNPCs[npc.ObjectID] = true
		}
	}

	// Send DeleteObject for NPCs leaving visibility range
	removedCount := 0
	for objID := range playerState.KnownNPCs {
		if !currentVisible[objID] {
			deleteData := outclient.BuildDeleteObject(objID)
			if err := c.Send(deleteData); err != nil {
				log.Ctx(ctx).Warn().Err(err).
					Int32("npc_obj_id", objID).
					Msg("failed to send DeleteObject for NPC leaving range")
			} else {
				removedCount++
			}
			delete(playerState.KnownNPCs, objID)
		}
	}

	if newCount > 0 || removedCount > 0 {
		log.Ctx(ctx).Debug().
			Int32("char_id", playerState.CharID).
			Int("new_npcs", newCount).
			Int("removed_npcs", removedCount).
			Int("total_known", len(playerState.KnownNPCs)).
			Msg("NPC visibility updated")
	}
}
