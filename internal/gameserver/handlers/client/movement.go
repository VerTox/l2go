package client

import (
	"context"
	"fmt"
	"math"
	"sort"

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

	// Ground move cancels attack/interact intention in the game loop (stop chasing).
	h.gameLoopCmd <- gameloop.CmdMoveToLocation{CharID: playerState.CharID}

	logger.Debug().Msg("MoveToLocation confirmation sent to client")

	// Broadcast movement to visible players (excluding the moving player)
	if err := h.broadcastMovementToVisiblePlayers(ctx, result, c); err != nil {
		logger.Warn().Err(err).Msg("failed to broadcast movement")
		// Don't fail the movement for broadcast errors
	}

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

	// Drift correction is applied only while standing. During movement the position
	// is client-authoritative for ground walking (l2go-2ax): the registry sync +
	// player-visibility reconcile happen on the game loop (handlePlayerMoved), gated
	// by intention so combat/interact approach stays server-authoritative. Correcting
	// a moving client here is exactly what caused rubber-band.
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
	}

	// Notify game loop about position change (for active region tracking)
	h.gameLoopCmd <- gameloop.CmdPlayerMoved{
		CharID:   playerState.CharID,
		Position: clientPos,
	}

	// Update NPC visibility as player moves (use the client position directly).
	h.updateNPCVisibility(ctx, c, playerState, clientPos)

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

// moveBroadcastObserverCap bounds how many nearby players a single movement is
// broadcast to (l2go-pec). Movement fan-out is the dominant packet volume at high
// population (movers x observers ~ 1M+/s at a crowd) and, unlike batching, only
// capping the observer count cuts the actual packet COUNT (build + per-client XOR).
// Budget is movers x observers <= ~200K on the current host: with 5000 movers that
// needs <= ~40 observers, with 1000 movers <= 200 — so a single cap here keeps all
// tested densities in budget. Beyond it, distant players in a crowd don't see this
// mover animate until it stops/respawns their view — retail-acceptable, and the
// only lever that fits the volume budget. Tunable by load measurement.
const moveBroadcastObserverCap = 48

// closestPlayers returns at most k of the given players, the ones nearest to pos.
// Ordering is by squared distance (no sqrt needed). It sorts the input slice in
// place — the caller owns a freshly built VisiblePlayers slice, so this is safe.
func closestPlayers(players []*registry.PlayerWorldState, pos models.Position, k int) []*registry.PlayerWorldState {
	if len(players) <= k {
		return players
	}
	sort.Slice(players, func(i, j int) bool {
		return sqDist(players[i].Position, pos) < sqDist(players[j].Position, pos)
	})
	return players[:k]
}

// sqDist is the squared 2D distance between two positions (int64 to avoid overflow
// at map scale). Used only for ordering, so the sqrt is unnecessary.
func sqDist(a, b models.Position) int64 {
	dx := int64(a.X - b.X)
	dy := int64(a.Y - b.Y)
	return dx*dx + dy*dy
}

// broadcastMovementToVisiblePlayers broadcasts movement to nearby players, capped to
// the closest moveBroadcastObserverCap of them (l2go-pec) to bound the fan-out.
func (h *Handler) broadcastMovementToVisiblePlayers(ctx context.Context, result *usecase.MovementResult, excludeClient *client.ClientConn) error {
	if len(result.VisiblePlayers) == 0 {
		return nil // No players to notify
	}

	// Cap the fan-out to the nearest observers around the mover's start position.
	observers := closestPlayers(result.VisiblePlayers, result.StartPosition, moveBroadcastObserverCap)

	logger := log.Ctx(ctx).With().
		Int32("moving_char", result.CharacterID).
		Int("visible_count", len(result.VisiblePlayers)).
		Int("observer_count", len(observers)).
		Logger()

	// Create MoveToLocation packet for broadcasting
	movePacket := outclient.NewMoveToLocation(
		result.CharacterID,
		int32(result.TargetPosition.X), int32(result.TargetPosition.Y), int32(result.TargetPosition.Z),
		int32(result.StartPosition.X), int32(result.StartPosition.Y), int32(result.StartPosition.Z),
	)

	packetData := movePacket.GetData()
	broadcastCount := 0

	// Broadcast to the capped observer set.
	for _, visiblePlayer := range observers {
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

// findClientByAccount finds a client connection by account name via the thread-safe
// ConnectionRegistry (O(1)). It previously iterated h.sessions without holding
// sessionsMu, which raced with connect/disconnect writes and fatally crashed the
// whole process under load ("concurrent map iteration and map write"). The registry
// is keyed by the same normalized account name that setSession registers. (l2go-7c0)
func (h *Handler) findClientByAccount(accountName string) *client.ClientConn {
	return h.connections.GetConnection(accountName)
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

// Dynamic player-to-player visibility now lives in the game loop
// (reconcilePlayerVisibility), which owns every player's KnownPlayers set and drives
// spawn/despawn off the authoritative server position. (l2go-23g)

// updateNPCVisibility sends NpcInfo for newly visible NPCs and DeleteObject
// for NPCs that left the visibility range. Called during position updates.
func (h *Handler) updateNPCVisibility(ctx context.Context, c *client.ClientConn, playerState *registry.PlayerWorldState, pos models.Position) {
	// Keep-set: NPCs within the forget radius stay spawned (hysteresis band). NpcInfo
	// is only sent for NPCs within the smaller watch radius. Same watch/forget radii
	// as player visibility (gameloop), so players and NPCs appear at the same distance.
	// pos is the client-reported position (l2go-2ax): during ground walking the
	// registry Position is synced by the loop, so we use the packet position directly
	// to keep NPC visibility exact on the connection goroutine.
	keep := make(map[int32]bool)
	for _, npc := range h.world.GetNPCsInRange(pos, registry.VisibilityForgetRadius) {
		keep[npc.ObjectID] = true
	}

	// Send NpcInfo for NPCs entering the watch radius
	newCount := 0
	for _, npc := range h.world.GetNPCsInRange(pos, registry.VisibilityWatchRadius) {
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

	// Send DeleteObject for NPCs beyond the forget radius
	removedCount := 0
	for objID := range playerState.KnownNPCs {
		if !keep[objID] {
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
