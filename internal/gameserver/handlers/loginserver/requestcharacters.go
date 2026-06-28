package loginserver

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
)

func (h *Handler) handleRequestCharacters(ctx context.Context, data []byte) error {
	log.Ctx(ctx).Debug().Msg("Received RequestCharacters packet from LoginServer")

	packet := inls.NewRequestCharacters(data)
	if packet == nil {
		return fmt.Errorf("failed to parse RequestCharacters packet")
	}

	// Process through use case
	err := h.usc.HandleRequestCharacters(ctx, packet)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", packet.GetAccount()).
			Msg("Failed to process RequestCharacters packet")
		return err
	}

	log.Ctx(ctx).Debug().
		Str("account", packet.GetAccount()).
		Msg("RequestCharacters packet processed successfully")

	return nil
}