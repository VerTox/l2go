package client

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// handleMultiPacket processes 0xd0 multi-packet opcodes with sub-opcodes
func (h *Handler) handleMultiPacket(ctx context.Context, c *client.ClientConn, payload []byte) error {
	if len(payload) < 2 { // Need at least 1 byte opcode + 2 bytes sub-opcode
		log.Ctx(ctx).Warn().Msg("multi-packet too short for sub-opcode")
		return nil
	}

	// Extract sub-opcode (2 bytes after the 0xd0 opcode)
	subOpcode := payload[0]
	subPayload := payload[1:] // Rest of the packet after sub-opcode

	log.Ctx(ctx).Debug().
		Str("sub_opcode", fmt.Sprintf("0x%x", subOpcode)).
		Int("payload_len", len(subPayload)).
		Msg("multi-packet received")

	switch subOpcode {
	case 0x01: // RequestManorList
		return nil
	case 0x0d: // RequestAutoSoulShot
		return h.handleRequestAutoSoulShot(ctx, c, subPayload)
	case 0x21: // RequestKeyMapping
		return h.handleRequestKeyMapping(ctx, c, subPayload)
	case 0x22: // RequestSaveKeyMapping
		return h.handleRequestSaveKeyMapping(ctx, c, subPayload)
	case 0x24: // Unknown packet - likely related to UI or settings
		log.Ctx(ctx).Debug().
			Int("payload_len", len(subPayload)).
			Msg("Unknown 0x24 multi-packet - ignoring for now")
		return nil
	case 0x36: // RequestGotoLobby - return to character selection after character creation  
		return h.handleRequestGotoLobby(ctx, c, subPayload)
	case 0x38: // RequestGotoLobby - alternative opcode (Java L2J uses D0:38)
		return h.handleRequestGotoLobby(ctx, c, subPayload)
	default:
		log.Ctx(ctx).Debug().
			Str("sub_opcode", fmt.Sprintf("0x%x", subOpcode)).
			Msg("unimplemented multi-packet sub-opcode")
		return nil
	}
}

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
