package loginserver

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
)

func (h *Handler) handleChangePasswordResponse(ctx context.Context, data []byte) error {
	log.Ctx(ctx).Debug().Msg("Received ChangePasswordResponse packet from LoginServer")

	packet := inls.NewChangePasswordResponse(data)
	if packet == nil {
		return fmt.Errorf("failed to parse ChangePasswordResponse packet")
	}

	// Process through use case
	err := h.usc.HandleChangePasswordResponse(ctx, packet)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", packet.GetAccount()).
			Msg("Failed to process ChangePasswordResponse packet")
		return err
	}

	if packet.HasChanged() {
		log.Ctx(ctx).Info().
			Str("account", packet.GetAccount()).
			Msg("Password change successful")
	} else {
		log.Ctx(ctx).Warn().
			Str("account", packet.GetAccount()).
			Msg("Password change failed")
	}

	return nil
}