package loginserver

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
)

func (h *Handler) handleAuthResponse(ctx context.Context, data []byte) error {
	log.Ctx(ctx).Info().Msg("Received AuthResponse packet from LoginServer")

	packet := inls.NewAuthResponse(data)
	if packet == nil {
		return fmt.Errorf("failed to parse AuthResponse packet")
	}

	// Update handler state
	h.mu.Lock()
	h.serverID = packet.GetServerID()
	h.serverName = packet.GetServerName()
	h.mu.Unlock()

	// Process through use case
	err := h.usc.HandleAuthResponse(ctx, packet)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed to process AuthResponse packet")
		return err
	}

	log.Ctx(ctx).Info().
		Int("server_id", packet.GetServerID()).
		Str("server_name", packet.GetServerName()).
		Msg("AuthResponse packet processed successfully - GameServer authenticated")

	return nil
}