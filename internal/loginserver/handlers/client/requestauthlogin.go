package client

import (
	"context"
	"encoding/binary"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets"
	"github.com/VerTox/l2go/internal/loginserver/packets/inclient"
	"github.com/VerTox/l2go/internal/loginserver/packets/outclient"
	"github.com/VerTox/l2go/internal/loginserver/transport"
	"github.com/VerTox/l2go/internal/loginserver/usecase"
)

func (h *Handler) handleRequestAuthLogin(ctx context.Context, client *transport.Client, data []byte) error {
	packet, err := inclient.NewRequestAuthLogin(ctx, client, data)
	if err != nil {
		client.Send(outclient.NewLoginFailPacket(packets.REASON_ACCESS_FAILED))
	}

	account, err := h.usc.HandleAuthLogin(ctx, packet)
	if err != nil {
		switch err {
		case usecase.ErrAccountBanned:
			client.Send(outclient.NewLoginFailPacket(packets.REASON_ACCOUNT_SUSPENDED_CALL))
		case usecase.ErrAccountNotFound:
			client.Send(outclient.NewLoginFailPacket(packets.REASON__PASS_WRONG))
		default:
			client.Send(outclient.NewLoginFailPacket(packets.REASON_ACCESS_FAILED))
		}

		return nil
	}

	client.Account = account

	client.AccessLevel = int(account.AccessLevel)
	client.LoginOkID1 = binary.LittleEndian.Uint32(client.SessionID[:4])
	client.LoginOkID2 = binary.LittleEndian.Uint32(client.SessionID[4:8])

	err = h.gameServerCommUseCase.RequestCharacterCountsFromAllServers(ctx, account.Username)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).
			Str("account", account.Username).
			Msg("Failed to request character counts after authentication")
	}

	client.Send(outclient.NewLoginOkPacket(client.SessionID))

	return nil
}
