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

// LogoutUseCase interface defines logout-related business logic
type LogoutUseCase interface {
	// Complete logout - disconnect from game entirely
	PerformLogout(ctx context.Context, accountName string, charID int32) error
	
	// Return to character selection (restart)
	PerformRestart(ctx context.Context, accountName string, charID int32) error
	
	// Return to lobby from character selection
	PerformGotoLobby(ctx context.Context, accountName string) error
	
	// Check if logout/restart is allowed (anti-exploit)
	CanPerformLogout(ctx context.Context, charID int32) (*LogoutValidation, error)
}

// LogoutValidation represents logout validation result
type LogoutValidation struct {
	CanLogout     bool   `json:"can_logout"`
	Reason        string `json:"reason,omitempty"`
	CharacterID   int32  `json:"character_id"`
	IsInCombat    bool   `json:"is_in_combat"`
	IsEnchanting  bool   `json:"is_enchanting"`
	IsFestival    bool   `json:"is_festival"`
	ForceAllowed  bool   `json:"force_allowed"` // GM or admin override
}

// logoutUseCase implements LogoutUseCase interface
type logoutUseCase struct {
	worldRegistry *registry.WorldRegistry
	charRepo      repo.CharacterRepository
	logger        zerolog.Logger
}

// NewLogoutUseCase creates a new logout use case
func NewLogoutUseCase(
	worldRegistry *registry.WorldRegistry,
	charRepo repo.CharacterRepository,
	logger zerolog.Logger,
) LogoutUseCase {
	return &logoutUseCase{
		worldRegistry: worldRegistry,
		charRepo:      charRepo,
		logger:        logger.With().Str("component", "logout").Logger(),
	}
}

// PerformLogout handles complete game logout
func (uc *logoutUseCase) PerformLogout(ctx context.Context, accountName string, charID int32) error {
	logger := uc.logger.With().
		Str("account", accountName).
		Int32("char_id", charID).
		Logger()

	logger.Info().Msg("performing complete logout")

	// 1. Validate logout is allowed
	validation, err := uc.CanPerformLogout(ctx, charID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to validate logout")
		return fmt.Errorf("failed to validate logout: %w", err)
	}

	if !validation.CanLogout && !validation.ForceAllowed {
		logger.Warn().Str("reason", validation.Reason).Msg("logout not allowed")
		return fmt.Errorf("logout not allowed: %s", validation.Reason)
	}

	// 2. Save character position and data to database
	if err := uc.saveCharacterData(ctx, charID); err != nil {
		logger.Error().Err(err).Msg("failed to save character data")
		// Continue with logout even if save fails
	}

	// 3. Remove character from world
	if err := uc.removeCharacterFromWorld(ctx, charID, accountName); err != nil {
		logger.Error().Err(err).Msg("failed to remove character from world")
		// Continue with logout even if world removal fails
	}

	logger.Info().Msg("complete logout successful")
	return nil
}

// PerformRestart handles return to character selection from in-game
func (uc *logoutUseCase) PerformRestart(ctx context.Context, accountName string, charID int32) error {
	logger := uc.logger.With().
		Str("account", accountName).
		Int32("char_id", charID).
		Logger()

	logger.Info().Msg("performing restart to character selection")

	// 1. Validate restart is allowed (same rules as logout)
	validation, err := uc.CanPerformLogout(ctx, charID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to validate restart")
		return fmt.Errorf("failed to validate restart: %w", err)
	}

	if !validation.CanLogout && !validation.ForceAllowed {
		logger.Warn().Str("reason", validation.Reason).Msg("restart not allowed")
		return fmt.Errorf("restart not allowed: %s", validation.Reason)
	}

	// 2. Save character position and data to database
	if err := uc.saveCharacterData(ctx, charID); err != nil {
		logger.Error().Err(err).Msg("failed to save character data during restart")
		// Continue with restart even if save fails
	}

	// 3. Remove character from world (but keep session active)
	if err := uc.removeCharacterFromWorld(ctx, charID, accountName); err != nil {
		logger.Error().Err(err).Msg("failed to remove character from world during restart")
		// Continue with restart even if world removal fails
	}

	logger.Info().Msg("restart to character selection successful")
	return nil
}

// PerformGotoLobby handles return to lobby from character selection
func (uc *logoutUseCase) PerformGotoLobby(ctx context.Context, accountName string) error {
	logger := uc.logger.With().
		Str("account", accountName).
		Logger()

	logger.Info().Msg("performing goto lobby")

	// GotoLobby is simple - just return to character selection
	// No character to save or remove from world since already in character selection

	logger.Debug().Msg("goto lobby successful")
	return nil
}

// CanPerformLogout validates if logout/restart is allowed
func (uc *logoutUseCase) CanPerformLogout(ctx context.Context, charID int32) (*LogoutValidation, error) {
	logger := uc.logger.With().Int32("char_id", charID).Logger()

	// Get player state from world
	playerState, exists := uc.worldRegistry.GetPlayer(charID)
	if !exists {
		// Player not in world - logout is always allowed
		return &LogoutValidation{
			CanLogout:   true,
			CharacterID: charID,
		}, nil
	}

	validation := &LogoutValidation{
		CanLogout:   true,
		CharacterID: charID,
		IsInCombat:  playerState.InCombat,
	}

	// Check combat state
	if playerState.InCombat {
		validation.CanLogout = false
		validation.Reason = "cannot logout while in combat"
		logger.Debug().Msg("logout prevented: in combat")
	}

	// TODO: Add more validation checks
	// - IsEnchanting: Check if player is enchanting items
	// - IsFestival: Check if player is in festival
	// - IsTrading: Check if player is trading
	// - IsInOlympiad: Check if player is in olympiad match
	
	// Check if player has admin/GM privileges (force allow)
	// TODO: Implement access level checking
	// if playerState.AccessLevel > 0 {
	//     validation.ForceAllowed = true
	// }

	return validation, nil
}

// Private helper methods

// saveCharacterData saves character data to database before logout/restart
func (uc *logoutUseCase) saveCharacterData(ctx context.Context, charID int32) error {
	logger := uc.logger.With().Int32("char_id", charID).Logger()

	// Skip save if charID is 0 (player not in world)
	if charID == 0 {
		logger.Debug().Msg("skipping character data save - player not in world")
		return nil
	}

	// Persist the LIVE in-world character: EXP, level, SP, HP/MP/CP are updated
	// in memory on playerState.Character during play. Reloading from the DB here
	// would discard all that progress and save only the position. Fall back to the
	// DB copy only when the character isn't in the world.
	var char *models.Character
	if playerState, exists := uc.worldRegistry.GetPlayer(charID); exists && playerState.Character != nil {
		char = playerState.Character
		char.Position = playerState.Position
		char.SetHeading(int(playerState.Heading))
	} else {
		var err error
		char, err = uc.charRepo.GetByID(ctx, charID)
		if err != nil {
			return fmt.Errorf("failed to get character: %w", err)
		}
	}

	// Update last access time
	char.LastAccess = time.Now().Unix()

	// Save to database
	if err := uc.charRepo.Update(ctx, char); err != nil {
		return fmt.Errorf("failed to update character: %w", err)
	}

	logger.Debug().
		Int("x", char.Position.X).
		Int("y", char.Position.Y).
		Int("z", char.Position.Z).
		Int64("exp", char.Experience).
		Int("level", char.Level).
		Int("sp", char.SP).
		Msg("character data saved")

	return nil
}

// removeCharacterFromWorld removes character from world registry and notifies other players
func (uc *logoutUseCase) removeCharacterFromWorld(ctx context.Context, charID int32, accountName string) error {
	logger := uc.logger.With().
		Int32("char_id", charID).
		Str("account", accountName).
		Logger()

	// Get player state before removal for cleanup
	playerState, exists := uc.worldRegistry.GetPlayer(charID)
	if !exists {
		logger.Debug().Msg("character not found in world registry")
		return nil // Already removed or never in world
	}

	// TODO: Notify visible players about character logout
	// visiblePlayers := uc.worldRegistry.GetPlayersInRange(playerState.Position, 4000)
	// for _, visiblePlayer := range visiblePlayers {
	//     // Send DeleteObject packet to visible players
	// }

	// Remove from world registry
	if err := uc.worldRegistry.RemovePlayer(ctx, charID); err != nil {
		logger.Error().Err(err).Msg("failed to remove player from world registry")
		return fmt.Errorf("failed to remove from world: %w", err)
	}

	logger.Info().
		Int("x", playerState.Position.X).
		Int("y", playerState.Position.Y).
		Int("z", playerState.Position.Z).
		Msg("character removed from world")

	return nil
}