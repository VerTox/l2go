package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/repo"
)

// MovementUseCase interface defines movement-related business logic
type MovementUseCase interface {
	// Movement operations
	StartMovement(ctx context.Context, charID int32, destination models.Position, isRunning bool) (*MovementResult, error)
	ValidateMovement(ctx context.Context, charID int32, from, to models.Position, isRunning bool) error
	UpdatePosition(ctx context.Context, charID int32, position models.Position, heading int32) error
	StopMovement(ctx context.Context, charID int32) error
	
	// Position synchronization
	ValidateClientPosition(ctx context.Context, charID int32, clientPos models.Position, heading int32) (*PositionCorrectionResult, error)
	GetCurrentPosition(ctx context.Context, charID int32) (*models.Position, error)
	
	// Visibility and broadcasting
	GetVisiblePlayers(ctx context.Context, charID int32) ([]*registry.PlayerWorldState, error)
	BroadcastMovement(ctx context.Context, charID int32, movement MovementBroadcast) error
}

// MovementResult represents the result of a movement operation
type MovementResult struct {
	Success         bool                  `json:"success"`
	CharacterID     int32                 `json:"character_id"`
	StartPosition   models.Position       `json:"start_position"`
	TargetPosition  models.Position       `json:"target_position"`
	EstimatedTime   time.Duration         `json:"estimated_time"`
	VisiblePlayers  []*registry.PlayerWorldState `json:"visible_players,omitempty"`
	ValidationError *MovementValidationError     `json:"validation_error,omitempty"`
}

// PositionCorrectionResult represents position validation result
type PositionCorrectionResult struct {
	NeedsCorrection bool            `json:"needs_correction"`
	ExpectedPos     models.Position `json:"expected_position"`
	ClientPos       models.Position `json:"client_position"`
	Deviation       float64         `json:"deviation"`
	CharacterID     int32           `json:"character_id"`
}

// MovementBroadcast represents movement data to broadcast to other players
type MovementBroadcast struct {
	CharacterID    int32           `json:"character_id"`
	StartPosition  models.Position `json:"start_position"`
	TargetPosition models.Position `json:"target_position"`
	IsRunning      bool            `json:"is_running"`
	Timestamp      time.Time       `json:"timestamp"`
}

// movementUseCase implements MovementUseCase interface
type movementUseCase struct {
	worldRegistry *registry.WorldRegistry
	charRepo      repo.CharacterRepository
	validator     *MovementValidator
	logger        zerolog.Logger
}

// NewMovementUseCase creates a new movement use case
func NewMovementUseCase(
	worldRegistry *registry.WorldRegistry,
	charRepo repo.CharacterRepository,
	logger zerolog.Logger,
) MovementUseCase {
	return &movementUseCase{
		worldRegistry: worldRegistry,
		charRepo:      charRepo,
		validator:     NewMovementValidator(),
		logger:        logger.With().Str("component", "movement").Logger(),
	}
}

// StartMovement initiates character movement
func (uc *movementUseCase) StartMovement(
	ctx context.Context,
	charID int32,
	destination models.Position,
	isRunning bool,
) (*MovementResult, error) {
	logger := uc.logger.With().
		Int32("char_id", charID).
		Int("dest_x", destination.X).
		Int("dest_y", destination.Y).
		Int("dest_z", destination.Z).
		Bool("is_running", isRunning).
		Logger()
	
	logger.Debug().Msg("starting movement")
	
	// 1. Get current player state
	playerState, exists := uc.worldRegistry.GetPlayer(charID)
	if !exists {
		return nil, fmt.Errorf("player not found in world: %d", charID)
	}
	
	// CRITICAL FIX: Stop any existing movement and get ACTUAL current position
	// This prevents speed bug when setting sequential waypoints
	if playerState.IsMoving {
		logger.Debug().Msg("interrupting existing movement")
		if err := uc.StopMovement(ctx, charID); err != nil {
			logger.Warn().Err(err).Msg("failed to stop existing movement")
			// Continue anyway - don't fail new movement
		}
		
		// Refresh player state after stopping movement
		if refreshedState, exists := uc.worldRegistry.GetPlayer(charID); exists {
			playerState = refreshedState
		}
		
		logger.Debug().
			Int("server_x", playerState.Position.X).
			Int("server_y", playerState.Position.Y).
			Msg("using server position after movement interruption")
	}
	
	currentPos := playerState.Position
	
	// 2. Validate movement request
	if err := uc.validator.ValidateMovementRequest(
		charID, currentPos, destination, isRunning,
	); err != nil {
		logger.Warn().Err(err).Msg("movement validation failed")
		
		return &MovementResult{
			Success:         false,
			CharacterID:     charID,
			StartPosition:   currentPos,
			TargetPosition:  destination,
			ValidationError: err.(*MovementValidationError),
		}, nil // Return validation error in result, not as error
	}
	
	// 3. Check if movement is significant enough to process
	if !IsSignificantMovement(currentPos, destination, 10.0) {
		logger.Debug().Msg("movement distance too small, ignoring")
		return &MovementResult{
			Success:        true,
			CharacterID:    charID,
			StartPosition:  currentPos,
			TargetPosition: currentPos, // No movement
			EstimatedTime:  0,
		}, nil
	}
	
	// 4. Update world registry with movement state
	if err := uc.updateMovementState(ctx, charID, destination, isRunning); err != nil {
		logger.Error().Err(err).Msg("failed to update movement state")
		return nil, fmt.Errorf("failed to update movement state: %w", err)
	}
	
	// 5. Get visible players for broadcasting
	visiblePlayers, err := uc.GetVisiblePlayers(ctx, charID)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to get visible players")
		visiblePlayers = []*registry.PlayerWorldState{} // Continue without broadcasting
	}
	
	// 6. Calculate estimated movement time
	distance := uc.validator.CalculateDistance(currentPos, destination)
	estimatedTime := CalculateMovementTime(distance, isRunning)
	
	logger.Info().
		Float64("distance", distance).
		Dur("estimated_time", estimatedTime).
		Int("visible_players", len(visiblePlayers)).
		Msg("movement started successfully")
	
	return &MovementResult{
		Success:        true,
		CharacterID:    charID,
		StartPosition:  currentPos,
		TargetPosition: destination,
		EstimatedTime:  estimatedTime,
		VisiblePlayers: visiblePlayers,
	}, nil
}

// ValidateMovement validates a movement request
func (uc *movementUseCase) ValidateMovement(
	ctx context.Context,
	charID int32,
	from, to models.Position,
	isRunning bool,
) error {
	return uc.validator.ValidateMovementRequest(charID, from, to, isRunning)
}

// UpdatePosition updates character position in world and database
func (uc *movementUseCase) UpdatePosition(
	ctx context.Context,
	charID int32,
	position models.Position,
	heading int32,
) error {
	logger := uc.logger.With().
		Int32("char_id", charID).
		Int("x", position.X).
		Int("y", position.Y).
		Int("z", position.Z).
		Int32("heading", heading).
		Logger()
	
	logger.Debug().Msg("updating position")
	
	// 1. Update world registry
	if err := uc.worldRegistry.UpdatePlayerPosition(ctx, charID, position, heading); err != nil {
		logger.Error().Err(err).Msg("failed to update world registry")
		return fmt.Errorf("failed to update world position: %w", err)
	}
	
	// 2. Update database (async for performance)
	go func() {
		ctx := context.Background()
		if err := uc.updateDatabasePosition(ctx, charID, position, heading); err != nil {
			logger.Error().Err(err).Msg("failed to update database position")
		}
	}()
	
	logger.Debug().Msg("position updated successfully")
	return nil
}

// StopMovement stops character movement and calculates current position
func (uc *movementUseCase) StopMovement(ctx context.Context, charID int32) error {
	logger := uc.logger.With().Int32("char_id", charID).Logger()
	
	// Get current player state
	playerState, exists := uc.worldRegistry.GetPlayer(charID)
	if !exists {
		return fmt.Errorf("player not found: %d", charID)
	}
	
	// CRITICAL FIX: When movement is interrupted, don't calculate position based on old movement data
	// Instead, rely on client's ValidatePosition packets for accurate position updates
	if playerState.IsMoving {
		logger.Debug().Msg("movement interrupted - keeping current validated position")
		
		// Calculate how much time has elapsed since movement started
		elapsedTime := time.Since(playerState.MoveStarted)
		
		logger.Debug().
			Dur("elapsed_time", elapsedTime).
			Int("current_x", playerState.Position.X).
			Int("current_y", playerState.Position.Y).
			Msg("movement interrupted - using current position from ValidatePosition updates")
			
		// Keep current position - it's already updated by ValidatePosition packets
		// Don't try to interpolate as it causes wrong position calculations
	}
	
	// Stop movement in world registry
	playerState.IsMoving = false
	playerState.MoveStartPos = models.Position{}    // Clear movement state
	playerState.MoveDestination = models.Position{} // Clear movement state
	playerState.LastUpdate = time.Now()
	
	logger.Debug().Msg("movement stopped")
	return nil
}

// ValidateClientPosition validates client-reported position against server expectation
func (uc *movementUseCase) ValidateClientPosition(
	ctx context.Context,
	charID int32,
	clientPos models.Position,
	heading int32,
) (*PositionCorrectionResult, error) {
	logger := uc.logger.With().
		Int32("char_id", charID).
		Int("client_x", clientPos.X).
		Int("client_y", clientPos.Y).
		Int("client_z", clientPos.Z).
		Logger()
	
	// Get expected position from server
	playerState, exists := uc.worldRegistry.GetPlayer(charID)
	if !exists {
		return nil, fmt.Errorf("player not found: %d", charID)
	}
	
	expectedPos := playerState.Position
	
	// Calculate deviation
	deviation := uc.validator.CalculateDistance(expectedPos, clientPos)
	
	needsCorrection := uc.validator.ShouldCorrectPosition(expectedPos, clientPos)
	
	if needsCorrection {
		logger.Warn().
			Float64("deviation", deviation).
			Float64("correction_threshold", uc.validator.GetCorrectionThreshold()).
			Msg("position correction needed")
	} else {
		logger.Debug().
			Float64("deviation", deviation).
			Msg("position validation passed")
	}
	
	return &PositionCorrectionResult{
		NeedsCorrection: needsCorrection,
		ExpectedPos:     expectedPos,
		ClientPos:       clientPos,
		Deviation:       deviation,
		CharacterID:     charID,
	}, nil
}

// GetCurrentPosition returns current character position
func (uc *movementUseCase) GetCurrentPosition(ctx context.Context, charID int32) (*models.Position, error) {
	playerState, exists := uc.worldRegistry.GetPlayer(charID)
	if !exists {
		return nil, fmt.Errorf("player not found: %d", charID)
	}
	
	return &playerState.Position, nil
}

// GetVisiblePlayers returns players visible to the specified character
func (uc *movementUseCase) GetVisiblePlayers(ctx context.Context, charID int32) ([]*registry.PlayerWorldState, error) {
	const VISIBILITY_RADIUS = 4000 // L2J standard visibility radius
	
	playerState, exists := uc.worldRegistry.GetPlayer(charID)
	if !exists {
		return nil, fmt.Errorf("player not found: %d", charID)
	}
	
	// Get all players in visibility range
	visiblePlayers := uc.worldRegistry.GetPlayersInRange(playerState.Position, VISIBILITY_RADIUS)
	
	// Filter out self
	var result []*registry.PlayerWorldState
	for _, player := range visiblePlayers {
		if player.CharID != charID {
			result = append(result, player)
		}
	}
	
	return result, nil
}

// BroadcastMovement broadcasts movement to visible players
func (uc *movementUseCase) BroadcastMovement(
	ctx context.Context,
	charID int32,
	movement MovementBroadcast,
) error {
	logger := uc.logger.With().Int32("char_id", charID).Logger()
	
	visiblePlayers, err := uc.GetVisiblePlayers(ctx, charID)
	if err != nil {
		return fmt.Errorf("failed to get visible players: %w", err)
	}
	
	logger.Debug().
		Int("visible_players", len(visiblePlayers)).
		Msg("broadcasting movement to visible players")
	
	// Note: Actual packet broadcasting will be handled by the handler layer
	// This method just prepares the data for broadcasting
	
	return nil
}

// Private helper methods

// updateMovementState updates movement state in world registry
func (uc *movementUseCase) updateMovementState(
	ctx context.Context,
	charID int32,
	destination models.Position,
	isRunning bool,
) error {
	playerState, exists := uc.worldRegistry.GetPlayer(charID)
	if !exists {
		return fmt.Errorf("player not found: %d", charID)
	}
	
	// Update movement state
	currentPos := playerState.Position         // Get current position
	playerState.IsMoving = true
	playerState.MoveStarted = time.Now()
	playerState.MoveStartPos = currentPos      // Store starting position
	playerState.MoveDestination = destination  // Store destination
	playerState.LastUpdate = time.Now()
	
	// CRITICAL FIX: Do NOT update position to destination immediately
	// Player should stay at current position and move gradually
	// Position will be updated when client sends ValidatePosition or CannotMoveAnymore
	
	// Just update movement state - keep current position unchanged
	return nil
}

// updateDatabasePosition updates character position in database
func (uc *movementUseCase) updateDatabasePosition(
	ctx context.Context,
	charID int32,
	position models.Position,
	heading int32,
) error {
	// Get character from database
	char, err := uc.charRepo.GetByID(ctx, charID)
	if err != nil {
		return fmt.Errorf("failed to get character: %w", err)
	}
	
	// Update position
	char.Position = position
	char.SetHeading(int(heading))
	
	// Save to database
	if err := uc.charRepo.Update(ctx, char); err != nil {
		return fmt.Errorf("failed to update character position: %w", err)
	}
	
	return nil
}

// calculatePositionDuringMovement calculates the current position during movement
// based on elapsed time, speed, and linear interpolation
func (uc *movementUseCase) calculatePositionDuringMovement(
	playerState *registry.PlayerWorldState, 
	elapsedTime time.Duration,
) models.Position {
	// If no movement data available, return current position
	if !playerState.IsMoving || elapsedTime <= 0 {
		return playerState.Position
	}
	
	startPos := playerState.MoveStartPos
	destPos := playerState.MoveDestination
	
	// Calculate total distance and expected movement time
	distance := uc.validator.CalculateDistance(startPos, destPos)
	expectedTime := CalculateMovementTime(distance, playerState.IsRunning)
	
	// Calculate movement progress (0.0 to 1.0)
	progress := float64(elapsedTime) / float64(expectedTime)
	
	// Cap progress at 1.0 (arrived at destination)
	if progress > 1.0 {
		progress = 1.0
	}
	
	// Linear interpolation between start and destination
	currentX := float64(startPos.X) + (float64(destPos.X-startPos.X) * progress)
	currentY := float64(startPos.Y) + (float64(destPos.Y-startPos.Y) * progress)
	currentZ := float64(startPos.Z) + (float64(destPos.Z-startPos.Z) * progress)
	
	return models.Position{
		X: int(currentX),
		Y: int(currentY),
		Z: int(currentZ),
	}
}