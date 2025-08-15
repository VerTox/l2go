package client

import (
	"context"

	"github.com/VerTox/l2go/internal/loginserver/packets/outclient"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) sendInitPacket(ctx context.Context, client *transport.Client) error {
	init := outclient.NewInitPacket(client)

	return client.SendStatic(init)
}
