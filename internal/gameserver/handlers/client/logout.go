package client

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// handleLogout processes complete game logout (opcode 0x00)
func (h *Handler) handleLogout(ctx context.Context, c *client.ClientConn, payload []byte) error {
	_, err := inclient.ParseLogout(payload)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to parse Logout packet")
		return err
	}

	session := h.getSession(c)
	if session == nil {
		log.Ctx(ctx).Warn().Msg("logout request without valid session")
		return nil
	}

	logger := log.Ctx(ctx).With().
		Str("account", session.AccountName).
		Logger()

	logger.Info().Msg("processing logout request")

	// Get character ID from world registry (if player is in game)
	var charID int32 = 0
	playerState, playerExists := h.world.GetPlayerByAccount(session.AccountName)
	if playerExists {
		charID = playerState.CharID
		logger.Debug().
			Int32("char_id", charID).
			Str("char_name", playerState.Character.Name).
			Msg("player found in world for logout")
	}

	// Check if logout is allowed (combat, trading, etc.)
	if charID > 0 {
		validation, err := h.logoutUseCase.CanPerformLogout(ctx, charID)
		if err != nil {
			logger.Error().Err(err).Msg("failed to validate logout")
			// Continue with forced logout
		} else if !validation.CanLogout && !validation.ForceAllowed {
			logger.Info().
				Str("reason", validation.Reason).
				Msg("logout not allowed - blocking logout in combat")
			// Send "You may not log out while in combat" message
			msg := outclient.BuildSystemMessageNoParams(outclient.SysMsgCannotLogoutInCombat)
			_ = c.Send(msg)
			// Send ActionFailed to unblock client
			_ = c.Send(outclient.BuildActionFailed())
			return nil
		}
	}

	// Get player state for despawn broadcast BEFORE removing from world
	if playerState, exists := h.world.GetPlayer(charID); exists {
		// Broadcast DeleteObject to nearby players
		h.broadcastPlayerDespawn(ctx, playerState)
		logger.Debug().Msg("player despawn broadcasted to nearby players")
	} else {
		logger.Debug().Msg("player not in world registry - no despawn broadcast needed")
	}

	// Perform graceful logout through use case
	if err := h.logoutUseCase.PerformLogout(ctx, session.AccountName, charID); err != nil {
		logger.Error().Err(err).Msg("logout failed")
		// Continue with client disconnect even if logout fails
	}

	// Send LeaveWorld packet for graceful client exit
	// This prevents the "Connection lost" dialog and allows clean client shutdown
	leaveWorld := outclient.NewLeaveWorld()
	if err := c.Send(leaveWorld.GetData()); err != nil {
		logger.Error().Err(err).Msg("failed to send LeaveWorld packet")
		// Continue with connection close even if packet send fails
	} else {
		logger.Debug().Msg("LeaveWorld packet sent successfully")
	}

	// Give client time to process the LeaveWorld packet before closing connection
	// Based on Java L2J and TypeScript implementations
	time.Sleep(500 * time.Millisecond)

	logger.Info().Msg("logout completed - closing connection")
	
	// Close connection after LeaveWorld packet delivery
	c.Close()
	return nil
}

// handleRequestRestart processes restart to character selection (opcode 0x57)
func (h *Handler) handleRequestRestart(ctx context.Context, c *client.ClientConn, payload []byte) error {
	_, err := inclient.ParseRequestRestart(payload)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to parse RequestRestart packet")
		return err
	}

	session := h.getSession(c)
	if session == nil {
		log.Ctx(ctx).Warn().Msg("restart request without valid session")
		// Send failure response
		restartResponse := outclient.NewRestartResponse(false)
		return c.Send(restartResponse.GetData())
	}

	logger := log.Ctx(ctx).With().
		Str("account", session.AccountName).
		Logger()

	logger.Info().Msg("processing restart request")

	// Get character ID from world registry
	var charID int32 = 0
	if playerState, exists := h.world.GetPlayerByAccount(session.AccountName); exists {
		charID = playerState.CharID
	}

	// Perform restart through use case
	if err := h.logoutUseCase.PerformRestart(ctx, session.AccountName, charID); err != nil {
		logger.Error().Err(err).Msg("restart failed")
		
		// Send failure response
		restartResponse := outclient.NewRestartResponse(false)
		return c.Send(restartResponse.GetData())
	}

	logger.Info().Msg("restart successful")

	// CRITICAL FIX: Session management for restart
	// Based on Java L2J: client.setState(GameClientState.AUTHED) 
	// Session must stay alive (can select characters) but character should be removed from world
	// The use case already removed character from world registry
	// Session data (AccountName, SessionID, Keys) remains intact for character selection
	
	logger.Debug().
		Str("account", session.AccountName).
		Uint32("session_id", session.SessionID).
		Msg("session preserved for restart - character removed from world")

	// Send success response
	restartResponse := outclient.NewRestartResponse(true)
	if err := c.Send(restartResponse.GetData()); err != nil {
		logger.Error().Err(err).Msg("failed to send RestartResponse")
		return err
	}

	logger.Debug().Msg("RestartResponse sent successfully")

	// CRITICAL FIX: Force refresh character data from database for restart
	// Based on TypeScript implementation: await client.updateCharacterSelection()
	// This prevents character list corruption after restart
	logger.Debug().Msg("refreshing character list from database for restart")
	characters, err := h.characterUseCase.GetCharacterListEntries(ctx, session.AccountName)
	if err != nil {
		logger.Error().Err(err).Msg("failed to refresh character list from database for restart")
		return err
	}

	// Convert domain models to packet format (FIXED: match auth.go exactly)
	chars := make([]outclient.CharSelectInfoPackage, len(characters))
	for i, char := range characters {
		// CharacterListEntry has embedded Character, so we can access fields directly
		chars[i] = outclient.CharSelectInfoPackage{
			Name:             char.Name,
			ObjectID:         char.ID,
			ClanID:           int32(char.ClanID),
			Sex:              int32(char.Sex),
			Race:             int32(char.Race),
			BaseClassID:      int32(char.BaseClass),
			ClassID:          int32(char.ClassID),
			X:                int32(char.Position.X),
			Y:                int32(char.Position.Y),
			Z:                int32(char.Position.Z),
			CurrentHp:        char.CurrentHP,
			CurrentMp:        char.CurrentMP,
			MaxHp:            float64(char.MaxHP),
			MaxMp:            float64(char.MaxMP),
			Sp:               int32(char.SP),
			Exp:              char.Experience,
			Level:            int32(char.Level),
			Karma:            int32(char.Karma),
			PkKills:          int32(char.PKKills),
			PvPKills:         int32(char.PvPKills),
			DeleteTimerMs:    char.DeleteTime,
			LastAccessMs:     char.LastAccess,
			VitalityPoints:   int32(char.VitalityPoints),
			PaperdollItemIDs: make([]int32, 26), // CRITICAL FIX: Missing in restart!
		}
	}

	charSelectionInfo := outclient.CharSelectionInfo{
		LoginName: session.AccountName,
		SessionID: int32(session.SessionID),
		ActiveIdx: -1, // No active character after restart
		Chars:     chars,
		CharConf: outclient.CharacterConfig{
			CharMaxNumber: 7,
		},
	}

	if err := c.Send(l2pkt.BuildPacket(charSelectionInfo)); err != nil {
		logger.Error().Err(err).Msg("failed to send CharSelectionInfo")
		return err
	}

	logger.Debug().Msg("CharSelectionInfo sent successfully")
	return nil
}


