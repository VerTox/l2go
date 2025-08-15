package gameserver

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets/ings"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handlePlayerTracert(ctx context.Context, gs *transport.GameServer, data []byte) error {
	if !gs.Authenticated {
		return errors.New("GameServer not authenticated")
	}

	log.Ctx(ctx).Info().
		Int("server_id", gs.ID).
		Str("remote_addr", gs.RemoteAddr()).
		Int("data_length", len(data)).
		Msg("Received PlayerTracert packet from GameServer")

	// Parse PlayerTracert packet
	playerTracert := ings.NewPlayerTracert(data)
	if playerTracert == nil {
		log.Ctx(ctx).Warn().
			Int("server_id", gs.ID).
			Hex("raw_data", data).
			Msg("Failed to parse PlayerTracert packet")
		return errors.New("failed to parse PlayerTracert packet")
	}

	account := playerTracert.GetAccount()
	pcIP := playerTracert.GetPCIP()
	hops := playerTracert.GetHops()

	log.Ctx(ctx).Info().
		Str("account", account).
		Str("pc_ip", pcIP).
		Strs("hops", hops).
		Int("server_id", gs.ID).
		Msg("Player traceroute information received")

	// TODO: Store traceroute information if needed
	// For now, just log the information - it's mainly used for debugging network issues

	return nil
}
