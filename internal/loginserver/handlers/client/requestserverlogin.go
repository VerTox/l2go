package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets/inclient"
	"github.com/VerTox/l2go/internal/loginserver/packets/outclient"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handleRequestServerLogin(ctx context.Context, client *transport.Client, data []byte) error {
	packet := inclient.NewRequestServerLogin(data)
	login, err := h.usc.HandleRequestServerLogin(ctx, &packet, client.Account, client.LoginOkID1, client.LoginOkID2)
	if err != nil {
		client.Send(outclient.NewLoginFailPacket(outclient.REASON_ACCESS_FAILED))

		return err
	}

	if !login.Success {
		client.Send(outclient.NewLoginFailPacket(outclient.REASON_ACCESS_FAILED))

		log.Ctx(ctx).Error().Msgf("Login failed for account , reason: %s", login.Reason)

		return nil
	}
	client.LastServer = int(packet.ServerID)
	client.Send(outclient.NewPlayOk(login.PlayKey1, login.PlayKey2).GetData())

	return nil
}
