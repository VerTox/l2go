package gameserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets/ings"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handlePlayerLogout(ctx context.Context, gs *transport.GameServer, data []byte) error {
	if !gs.Authenticated {
		return errors.New("GameServer not authenticated")
	}

	log.Ctx(ctx).Info().
		Int("server_id", gs.ID).
		Str("remote_addr", gs.RemoteAddr()).
		Int("data_length", len(data)).
		Msg("Received PlayerLogout packet from GameServer")

	// Parse PlayerLogout packet
	playerLogout := ings.NewPlayerLogout(data)
	if playerLogout == nil {
		log.Ctx(ctx).Warn().
			Int("server_id", gs.ID).
			Hex("raw_data", data).
			Msg("Failed to parse PlayerLogout packet")
		return errors.New("failed to parse PlayerLogout packet")
	}

	account := playerLogout.GetAccount()

	log.Ctx(ctx).Info().
		Str("account", account).
		Int("server_id", gs.ID).
		Msg("Player logged out from GameServer")

	err := h.usc.HandlePlayerLogout(ctx, account, gs.ID)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", account).
			Int("server_id", gs.ID).
			Msg("Failed to handle player logout")
		return fmt.Errorf("failed to handle player logout: %w", err)
	}

	log.Ctx(ctx).Info().
		Str("account", account).
		Int("server_id", gs.ID).
		Msg("Player logout processed successfully")

	return nil
}
