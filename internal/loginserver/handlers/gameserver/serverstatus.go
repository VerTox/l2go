package gameserver

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handleServerStatus(ctx context.Context, gs *transport.GameServer, data []byte) error {
	if !gs.Authenticated {
		return errors.New("GameServer not authenticated")
	}

	log.Ctx(ctx).Debug().
		Int("server_id", gs.ID).
		Str("remote_addr", gs.RemoteAddr()).
		Msg("Received ServerStatus packet from GameServer")

	// TODO: Parse actual status data and update registry
	// For now, just acknowledge receipt
	return nil
}
