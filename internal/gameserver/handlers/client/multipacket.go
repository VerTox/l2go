package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// Мультипакет 0xD0 диспетчеризуется через реестр (dispatch.go): sub-опкод
// читается как 2 байта (LE), а маппинг учитывает состояние соединения.
// Ниже — обработчики уже реализованных 0xD0 sub-пакетов.

// handleRequestAutoSoulShot toggles auto-use of a shot item (RequestAutoSoulShot,
// 0xD0:0x0d). On enable it registers the shot, echoes ExAutoSoulShot(on) + the
// "will be auto" message, and recharges immediately so the first swing benefits.
// On disable it drops the shot and echoes ExAutoSoulShot(off) + the cancel message.
// Fishing shots (6535-6540) are never automated. (l2go-btb)
func (h *Handler) handleRequestAutoSoulShot(ctx context.Context, c *client.ClientConn, payload []byte) error {
	packet := &inclient.RequestAutoSoulShot{}
	l2pkt.ParsePacket(payload, packet)

	session := h.getSession(c)
	if session == nil {
		return nil
	}
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		return nil
	}
	charID := playerState.CharID
	itemID := packet.ItemID

	// Fishing shots are not automated on retail.
	if itemID >= 6535 && itemID <= 6540 {
		return nil
	}

	if !packet.Activate {
		registry.GetAutoShotRegistry().Disable(charID, itemID)
		if err := c.Send(outclient.BuildExAutoSoulShot(itemID, 0)); err != nil {
			return err
		}
		return c.Send(outclient.NewSystemMessage(outclient.SysMsgAutoUseOfS1Cancelled).AddItemName(itemID).Build())
	}

	// Enable requires the shot to be in the player's inventory (L2J getItemByItemId).
	items, err := h.characterUseCase.GetCharacterAllItems(ctx, charID)
	if err != nil {
		return err
	}
	if !ownsItem(items, itemID) {
		return nil
	}

	registry.GetAutoShotRegistry().Enable(charID, itemID)
	if err := c.Send(outclient.BuildExAutoSoulShot(itemID, 1)); err != nil {
		return err
	}
	if err := c.Send(outclient.NewSystemMessage(outclient.SysMsgUseOfS1WillBeAuto).AddItemName(itemID).Build()); err != nil {
		return err
	}

	// Immediate recharge so the first swing is already charged (L2J calls
	// rechargeShots right after enabling). Runs on this connection goroutine.
	consumed, _, err := h.inventoryUseCase.RechargeAutoShots(ctx, charID)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Int32("char_id", charID).Msg("auto-shot enable recharge failed")
		return nil
	}
	h.SendInventoryUpdate(charID, consumed)
	return nil
}

// ownsItem reports whether the character holds at least one unit of itemID.
func ownsItem(items []models.CharacterItem, itemID int32) bool {
	for i := range items {
		if items[i].ItemID == itemID && items[i].Count > 0 {
			return true
		}
	}
	return false
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
