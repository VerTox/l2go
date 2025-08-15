package gameserver

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handlePlayerInGame(ctx context.Context, gs *transport.GameServer, data []byte) error {
	if !gs.Authenticated {
		return errors.New("GameServer not authenticated")
	}

	log.Ctx(ctx).Info().
		Int("server_id", gs.ID).
		Str("remote_addr", gs.RemoteAddr()).
		Int("data_length", len(data)).
		Msg("Received PlayerInGame packet from GameServer")

	// TODO: Parse the actual player list from the packet
	// For now, just log that players are entering the game
	// Packet format: [count:2bytes][accounts...] where each account is UTF-16 string

	if len(data) < 2 {
		log.Ctx(ctx).Warn().
			Int("server_id", gs.ID).
			Msg("PlayerInGame packet too short, ignoring")
		return nil
	}

	// Parse player count (first 2 bytes, little endian)
	playerCount := int(data[0]) | (int(data[1]) << 8)

	log.Ctx(ctx).Info().
		Int("server_id", gs.ID).
		Int("player_count", playerCount).
		Hex("raw_data", data).
		Msg("Players entered game on server")

	// TODO: Implement full parsing and online player tracking
	// This will be part of the PlayerInGame system implementation from CLAUDE.md

	return nil
}
