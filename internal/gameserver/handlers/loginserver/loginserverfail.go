package loginserver

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
)

func (h *Handler) handleLoginServerFail(ctx context.Context, data []byte) error {
	log.Ctx(ctx).Error().Msg("Received LoginServerFail packet from LoginServer")

	packet := inls.NewLoginServerFail(data)
	if packet == nil {
		return fmt.Errorf("failed to parse LoginServerFail packet")
	}

	// Process through use case
	err := h.usc.HandleLoginServerFail(ctx, packet)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("reason", packet.GetReason()).
			Msg("Failed to process LoginServerFail packet")
		return err
	}

	log.Ctx(ctx).Error().
		Str("reason", packet.GetReason()).
		Msg("LoginServer reported failure")

	return fmt.Errorf("login server failure: %s", packet.GetReason())
}