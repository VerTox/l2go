package loginserver

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
)

func (h *Handler) handleKickPlayer(ctx context.Context, data []byte) error {
	log.Ctx(ctx).Info().Msg("Received KickPlayer packet from LoginServer")

	packet := inls.NewKickPlayer(data)
	if packet == nil {
		return fmt.Errorf("failed to parse KickPlayer packet")
	}

	// Process through use case
	err := h.usc.HandleKickPlayer(ctx, packet)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", packet.GetAccount()).
			Msg("Failed to process KickPlayer packet")
		return err
	}

	log.Ctx(ctx).Info().
		Str("account", packet.GetAccount()).
		Msg("Player kicked by LoginServer request")

	return nil
}