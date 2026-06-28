package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// Мультипакет 0xD0 диспетчеризуется через реестр (dispatch.go): sub-опкод
// читается как 2 байта (LE), а маппинг учитывает состояние соединения.
// Ниже — обработчики уже реализованных 0xD0 sub-пакетов.

// handleRequestAutoSoulShot processes auto-shot configuration
func (h *Handler) handleRequestAutoSoulShot(ctx context.Context, c *client.ClientConn, payload []byte) error {
	packet := &inclient.RequestAutoSoulShot{}
	l2pkt.ParsePacket(payload, packet)

	log.Ctx(ctx).Debug().
		Int32("item_id", packet.ItemID).
		Bool("activate", packet.Activate).
		Msg("RequestAutoSoulShot packet")

	// TODO: Implement auto-shot logic
	// For now, just acknowledge the request
	return nil
}

// handleRequestKeyMapping processes key mapping request
func (h *Handler) handleRequestKeyMapping(ctx context.Context, c *client.ClientConn, payload []byte) error {
	log.Ctx(ctx).Debug().Msg("RequestKeyMapping packet")

	// TODO: Send current key mappings
	// For now, just acknowledge the request
	return nil
}

// handleRequestSaveKeyMapping processes key mapping save
func (h *Handler) handleRequestSaveKeyMapping(ctx context.Context, c *client.ClientConn, payload []byte) error {
	packet := &inclient.RequestSaveKeyMapping{}
	l2pkt.ParsePacket(payload, packet)

	log.Ctx(ctx).Debug().
		Int("data_len", len(packet.Data)).
		Msg("RequestSaveKeyMapping packet")

	// TODO: Save key mappings to database
	// For now, just acknowledge the request
	return nil
}
