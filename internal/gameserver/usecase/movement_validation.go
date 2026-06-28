package usecase

import (
	"fmt"
	"math"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// MovementValidator provides anti-cheat validation for character movement
// Based on Java L2J movement validation system
type MovementValidator struct {
	// Base movement speeds (L2J units per second)
	maxWalkSpeed float64
	maxRunSpeed  float64
	
	// L2J validation parameters
	maxMovementDistance   float64 // Maximum movement distance per request (L2J: 9900)
	correctionThreshold   float64 // Position correction threshold (L2J: ~500)
	maxPositionDeviation  float64 // Maximum acceptable position difference (L2J: ~600)
}

// NewMovementValidator creates a new movement validator with L2J-compatible settings
func NewMovementValidator() *MovementValidator {
	const (
		// L2J base movement speeds (units per second)
		baseWalkSpeed = 80.0  // Standard walking speed
		baseRunSpeed  = 120.0 // Standard running speed
		
		// L2J movement validation constants (from MoveBackwardToLocation.java)
		maxMovementDistance  = 9900.0 // Maximum movement distance per request
		correctionThreshold  = 500.0  // Position difference threshold for correction
		maxPositionDeviation = 600.0  // Maximum acceptable position difference
	)
	
	return &MovementValidator{
		maxWalkSpeed:         baseWalkSpeed,
		maxRunSpeed:          baseRunSpeed,
		maxMovementDistance:  maxMovementDistance,
		correctionThreshold:  correctionThreshold,
		maxPositionDeviation: maxPositionDeviation,
	}
}

// MovementValidationError represents movement validation errors
type MovementValidationError struct {
	Type        string
	Message     string
	Expected    interface{}
	Actual      interface{}
	CharacterID int32
}

func (e *MovementValidationError) Error() string {
	return fmt.Sprintf("movement validation failed [%s]: %s (char: %d)", 
		e.Type, e.Message, e.CharacterID)
}

// Movement validation error types
const (
	ValidationErrorSpeed    = "SPEED_TOO_HIGH"
	ValidationErrorDistance = "DISTANCE_TOO_LARGE"
	ValidationErrorTeleport = "TELEPORT_DETECTED"
	ValidationErrorBounds   = "OUT_OF_BOUNDS"
	ValidationErrorTime     = "TIME_INVALID"
)

// ValidateMovementRequest validates a movement request from client
// Uses L2J-compatible validation: only checks teleport prevention, not speed
func (mv *MovementValidator) ValidateMovementRequest(
	charID int32,
	from, to models.Position,
	isRunning bool,
) error {
	// 1. Validate coordinates are within world bounds
	if err := mv.validateWorldBounds(charID, to); err != nil {
		return err
	}
	
	// 2. Calculate movement distance
	distance := mv.CalculateDistance(from, to)
	
	// 3. L2J anti-teleport check: maximum 9900 units per movement request
	// This is the only distance validation L2J does for movement requests
	if distance > mv.maxMovementDistance {
		return &MovementValidationError{
			Type:        ValidationErrorDistance,
			Message:     fmt.Sprintf("movement distance %.2f exceeds L2J limit %.2f (anti-teleport)", distance, mv.maxMovementDistance),
			Expected:    mv.maxMovementDistance,
			Actual:      distance,
			CharacterID: charID,
		}
	}
	
	// Note: L2J does NOT validate speed per tick - it allows continuous movement
	// Speed validation is handled by position synchronization via ValidatePosition packets
	
	return nil
}

// ValidatePosition validates a position update against expected position
// Uses L2J ValidatePosition.java logic for position synchronization
func (mv *MovementValidator) ValidatePosition(
	charID int32,
	serverPos, clientPos models.Position,
) error {
	distance := mv.CalculateDistance(serverPos, clientPos)
	
	// L2J logic: beyond 600 units difference indicates likely speed hacking
	if distance > mv.maxPositionDeviation {
		return &MovementValidationError{
			Type:        ValidationErrorTeleport,
			Message:     fmt.Sprintf("position deviation %.2f exceeds L2J maximum %.2f (likely speed hack)", distance, mv.maxPositionDeviation),
			Expected:    serverPos,
			Actual:      clientPos,
			CharacterID: charID,
		}
	}
	
	return nil
}

// ShouldCorrectPosition determines if position should be forcibly corrected
// Uses L2J logic: correct when difference exceeds ~500 units
func (mv *MovementValidator) ShouldCorrectPosition(
	serverPos, clientPos models.Position,
) bool {
	distance := mv.CalculateDistance(serverPos, clientPos)
	return distance > mv.correctionThreshold
}

// GetCorrectionThreshold returns the correction threshold value
func (mv *MovementValidator) GetCorrectionThreshold() float64 {
	return mv.correctionThreshold
}

// Private helper methods

// validateWorldBounds checks if position is within L2J world bounds
func (mv *MovementValidator) validateWorldBounds(charID int32, pos models.Position) error {
	const (
		// L2J world boundaries (exact from Java L2J L2World.java)
		mapMinX = -294912  // (11 - 20) * 32768
		mapMaxX = 229376   // ((26 - 20) + 1) * 32768
		mapMinY = -262144  // (10 - 18) * 32768
		mapMaxY = 294912   // ((26 - 18) + 1) * 32768
		mapMinZ = -16384   // General Z bounds
		mapMaxZ = 16383
	)
	
	if pos.X < mapMinX || pos.X > mapMaxX ||
		pos.Y < mapMinY || pos.Y > mapMaxY ||
		pos.Z < mapMinZ || pos.Z > mapMaxZ {
		return &MovementValidationError{
			Type:        ValidationErrorBounds,
			Message:     fmt.Sprintf("position (%d,%d,%d) is outside world bounds", pos.X, pos.Y, pos.Z),
			Expected:    "valid world coordinates",
			Actual:      pos,
			CharacterID: charID,
		}
	}
	
	return nil
}

// CalculateDistance calculates 3D distance between two positions
func (mv *MovementValidator) CalculateDistance(from, to models.Position) float64 {
	dx := float64(to.X - from.X)
	dy := float64(to.Y - from.Y)
	dz := float64(to.Z - from.Z)
	
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// GetMaxMovementDistance returns the maximum allowed movement distance per request
func (mv *MovementValidator) GetMaxMovementDistance() float64 {
	return mv.maxMovementDistance
}

// Movement helper functions

// IsValidMovementType checks if movement type is valid
func IsValidMovementType(moveType int32) bool {
	return moveType >= 0 && moveType <= 1 // 0 = cursor keys, 1 = mouse
}

// IsSignificantMovement checks if movement is significant enough to process
func IsSignificantMovement(from, to models.Position, threshold float64) bool {
	dx := float64(to.X - from.X)
	dy := float64(to.Y - from.Y)
	dz := float64(to.Z - from.Z)
	
	distance := math.Sqrt(dx*dx + dy*dy + dz*dz)
	return distance >= threshold
}

// CalculateMovementTime estimates movement time based on distance and speed
func CalculateMovementTime(distance float64, isRunning bool) time.Duration {
	speed := 80.0 // walk speed
	if isRunning {
		speed = 120.0 // run speed
	}
	
	seconds := distance / speed
	return time.Duration(seconds * float64(time.Second))
}