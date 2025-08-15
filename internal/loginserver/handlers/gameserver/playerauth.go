package gameserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets/ings"
	"github.com/VerTox/l2go/internal/loginserver/packets/outgs"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handlePlayerAuthRequest(ctx context.Context, gs *transport.GameServer, data []byte) error {
	if !gs.Authenticated {
		return errors.New("GameServer not authenticated")
	}

	log.Ctx(ctx).Info().
		Int("server_id", gs.ID).
		Str("remote_addr", gs.RemoteAddr()).
		Msg("Received PlayerAuthRequest packet from GameServer")

	authReq := ings.NewPlayerAuthRequest(data)
	if authReq == nil {
		return errors.New("failed to parse PlayerAuthRequest packet")
	}

	// Process player authentication through use case
	result, err := h.usc.HandlePlayerAuthRequest(ctx, authReq)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", authReq.GetAccount()).
			Msg("Player authentication processing failed")
		return err
	}

	// Send PlayerAuthResponse
	response := outgs.NewPlayerAuthResponse(result.Account, result.Success)
	err = gs.Send(response.GetData())
	if err != nil {
		return fmt.Errorf("failed to send PlayerAuthResponse: %w", err)
	}

	log.Ctx(ctx).Info().
		Str("account", result.Account).
		Bool("success", result.Success).
		Str("reason", result.Reason).
		Int("server_id", gs.ID).
		Msg("Player authentication response sent")

	return nil
}
