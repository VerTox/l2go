package loginserver

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
)

func (h *Handler) handlePlayerAuthResponse(ctx context.Context, data []byte) error {
	log.Ctx(ctx).Debug().Msg("Received PlayerAuthResponse packet from LoginServer")

	packet := inls.NewPlayerAuthResponse(data)
	if packet == nil {
		return fmt.Errorf("failed to parse PlayerAuthResponse packet")
	}

	// Process through use case
	err := h.usc.HandlePlayerAuthResponse(ctx, packet)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", packet.GetAccount()).
			Msg("Failed to process PlayerAuthResponse packet")
		return err
	}

	if packet.IsAuthed() {
		log.Ctx(ctx).Info().
			Str("account", packet.GetAccount()).
			Msg("Player authentication successful")
	} else {
		log.Ctx(ctx).Warn().
			Str("account", packet.GetAccount()).
			Msg("Player authentication failed")
	}

	return nil
}