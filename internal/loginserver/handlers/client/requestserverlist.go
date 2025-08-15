package client

import (
	"bytes"
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets/inclient"
	"github.com/VerTox/l2go/internal/loginserver/packets/outclient"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handleServerList(ctx context.Context, client *transport.Client, data []byte) error {
	packet := inclient.NewRequestServerList(data)

	// Verify session ID to prevent unauthorized access
	if !bytes.Equal(client.SessionID[:8], packet.SessionID) {
		log.Ctx(ctx).Warn().
			Hex("expected", client.SessionID[:8]).
			Hex("received", packet.SessionID).
			Msg("Session ID mismatch for RequestServerList packet")

		// Send login fail for security violation
		loginFail := outclient.NewLoginFailPacket(outclient.REASON_ACCESS_FAILED)
		return client.Send(loginFail)
	}

	servers, err := h.gameServerUseCase.GetServerList(ctx, client.AccessLevel)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed to get server list")
		loginFail := outclient.NewLoginFailPacket(outclient.REASON_SYSTEM_ERROR)
		return client.Send(loginFail)
	}

	// Get cached character counts (they should have been requested during login)
	var characterCounts map[int]int
	if client.Account != nil {
		characterCounts = h.gameServerCommUseCase.GetCharacterCounts(ctx, client.Account.Username)
	}

	log.Ctx(ctx).Info().
		Str("client_addr", client.RemoteAddr()).
		Int("server_count", len(servers)).
		Int("access_level", client.AccessLevel).
		Int("char_counts", len(characterCounts)).
		Msg("Client requested server list")

	// Create and send server list response with character counts
	var serverList *outclient.ServerList
	if len(characterCounts) > 0 {
		serverList = outclient.NewServerListWithCharCounts(servers, client.LastServer, client.RemoteAddr(), client.AccessLevel, characterCounts)
	} else {
		serverList = outclient.NewServerList(servers, client.LastServer, client.RemoteAddr(), client.AccessLevel)
	}
	return client.Send(serverList.GetData())
}
