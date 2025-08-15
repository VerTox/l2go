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

func (h *Handler) handleAuthRequest(ctx context.Context, gs *transport.GameServer, data []byte) error {
	log.Ctx(ctx).Info().Str("remote_addr", gs.RemoteAddr()).Msg("Received AuthRequest packet from GameServer")

	authRequest := ings.NewAuthRequest(data)
	if authRequest == nil {
		return errors.New("failed to parse AuthRequest packet")
	}

	log.Ctx(ctx).Debug().Str("requst_hosts", fmt.Sprintf("%v", authRequest.GetHosts())).Msg("AuthRequest packet hosts")

	// Process authentication through use case
	result, err := h.usc.HandleAuthRequest(ctx, authRequest)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Int("server_id", authRequest.GetID()).Msg("GameServer authentication failed")
		return err
	}

	if !result.Success {
		log.Ctx(ctx).Warn().
			Int("server_id", authRequest.GetID()).
			Str("reason", result.Reason).
			Msg("GameServer authentication rejected")
		return fmt.Errorf("authentication rejected: %s", result.Reason)
	}

	// Store server ID and mark as authenticated
	gs.ID = result.ServerID
	gs.Authenticated = true

	// Send AuthResponse
	authResponse := outgs.NewAuthResponse(result.ServerID, result.ServerName)
	err = gs.Send(authResponse.GetData())
	if err != nil {
		return fmt.Errorf("failed to send AuthResponse: %w", err)
	}

	log.Ctx(ctx).Info().
		Int("server_id", result.ServerID).
		Str("server_name", result.ServerName).
		Str("remote_addr", gs.RemoteAddr()).
		Msg("GameServer authentication completed successfully")

	return nil
}
