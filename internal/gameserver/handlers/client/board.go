package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

// worldMapID — стандартная карта мира (mapId для ShowMiniMap), как в L2J HF
// (RequestShowMiniMap → new ShowMiniMap(1665)).
const worldMapID int32 = 1665

func init() { addStubRegistrator(registerBoardStubs) }

// registerBoardStubs регистрирует обработчики пакетов BBS-доски и мини-карты (High Five).
func registerBoardStubs(r *Registry) {
	// RequestBBSwrite (0x24): написать сообщение на BBS-доску (community board).
	r.registerStub(StateInGame, 0x24, "RequestBBSwrite")
	// RequestShowBoard (0x5e): открыть/показать BBS-доску.
	r.registerStub(StateInGame, 0x5e, "RequestShowBoard")
	// RequestShowMiniMap (0x6c): показать мини-карту региона — реальный обработчик.
	r.simple[StateInGame][0x6c] = packetEntry{
		Name:   "RequestShowMiniMap",
		Handle: (*Handler).handleRequestShowMiniMap,
	}
}

// handleRequestShowMiniMap отвечает на RequestShowMiniMap (0x6C) пакетом
// ShowMiniMap с картой мира. Тело входящего пакета пустое (trigger) — не читается.
func (h *Handler) handleRequestShowMiniMap(ctx context.Context, c *client.ClientConn, _ []byte) error {
	if err := c.Send(outclient.BuildShowMiniMap(worldMapID)); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to send ShowMiniMap")
		return err
	}
	return nil
}
