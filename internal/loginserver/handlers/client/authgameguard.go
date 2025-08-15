package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets"
	"github.com/VerTox/l2go/internal/loginserver/packets/inclient"
	"github.com/VerTox/l2go/internal/loginserver/packets/outclient"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handleAuthGameGuard(ctx context.Context, client *transport.Client, data []byte) error {
	authGG := inclient.NewAuthGameGuard(data)

	expectedSessionID := packets.GetSessionIdFromSessionBytes(client.SessionID)
	if authGG.GetSessionId() != expectedSessionID {
		log.Ctx(ctx).Error().Msgf("Invalid session ID in AuthGameGuard packet: expected %08X, got %08X",
			expectedSessionID, authGG.GetSessionId())

		return client.Send(outclient.NewLoginFailPacket(packets.REASON_ACCESS_FAILED))
	}

	authGGresp := outclient.NewGGAuthPacket(client)
	return client.Send(authGGresp)
}
