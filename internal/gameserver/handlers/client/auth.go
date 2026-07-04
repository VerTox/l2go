package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outls"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// handleAuthLogin processes authentication and sends character list
func (h *Handler) handleAuthLogin(ctx context.Context, c *client.ClientConn, payload []byte) error {
	packet := &inclient.AuthLogin{}
	l2pkt.ParsePacket(payload, packet)

	// Canonicalize the account name to lowercase at this single ingress point. The
	// LoginServer already lowercases logins, while the client sends the original
	// case; normalizing here keeps the session, created characters, DB queries and
	// registry map keys all in one canonical case. (l2go-xhp)
	account := models.NormalizeAccountName(packet.Account)
	log.Ctx(ctx).Info().Str("account", account).Msg("AuthLogin request")

	// Validate session keys with LoginServer
	if !h.validateSessionKeys(ctx, account, packet.LoginKey1, packet.LoginKey2, packet.PlayKey1, packet.PlayKey2) {
		log.Ctx(ctx).Warn().
			Str("account", account).
			Msg("session key validation failed")
		// TODO: Send authentication failure packet and disconnect
		return nil
	}

	// Create session for this client
	session := &ClientSession{
		AccountName: account,
		SessionID:   123456, // TODO: Generate proper session ID
		LoginKeys:   [2]uint32{uint32(packet.LoginKey1), uint32(packet.LoginKey2)},
		PlayKeys:    [2]uint32{uint32(packet.PlayKey1), uint32(packet.PlayKey2)},
	}
	// Kick any existing session for this account before registering the new one.
	h.kickExistingSession(ctx, account, c)

	h.setSession(c, session)

	// Load real characters from database
	characters, err := h.characterUseCase.GetCharacterListEntries(ctx, account)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("account", account).Msg("failed to load characters")
		// Send empty character list on error
		characters = []models.CharacterListEntry{}
	}

	// Convert domain models to packet format
	chars := make([]outclient.CharSelectInfoPackage, len(characters))
	for i, char := range characters {
		chars[i] = toCharSelectInfoPackage(char)
	}

	// Send character selection screen
	charSelectionInfo := outclient.CharSelectionInfo{
		LoginName: account,
		SessionID: int32(session.SessionID),
		ActiveIdx: -1, // No character selected initially
		Chars:     chars,
		CharConf: outclient.CharacterConfig{
			CharMaxNumber: 7, // Allow up to 7 characters per account
		},
	}

	if err := c.Send(l2pkt.BuildPacket(charSelectionInfo)); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to send character selection info")
		return err
	}

	log.Ctx(ctx).Info().
		Str("account", account).
		Int("character_count", len(characters)).
		Msg("Character list sent successfully")

	return nil
}

// validateSessionKeys validates session keys with LoginServer
func (h *Handler) validateSessionKeys(ctx context.Context, account string, loginKey1, loginKey2, playKey1, playKey2 int32) bool {
	sessionKey := outls.SessionKey{
		LoginOkID1: uint32(loginKey1),
		LoginOkID2: uint32(loginKey2),
		PlayOkID1:  uint32(playKey1),
		PlayOkID2:  uint32(playKey2),
	}

	// Send PlayerAuthRequest to LoginServer
	if err := h.loginServerHandler.SendPlayerAuthRequest(account, sessionKey); err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", account).
			Msg("failed to send PlayerAuthRequest to LoginServer")
		return false
	}

	// TODO: Wait for PlayerAuthResponse from LoginServer
	// For now, assume validation is successful
	log.Ctx(ctx).Info().
		Str("account", account).
		Msg("Session keys validated with LoginServer")

	return true
}
