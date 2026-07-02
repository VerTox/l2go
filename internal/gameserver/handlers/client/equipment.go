package client

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

// handleUseItem processes UseItem packet (opcode 0x19)
func (h *Handler) handleUseItem(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt := inclient.NewUseItem(payload)

	session := h.getSession(c)
	if session == nil {
		return fmt.Errorf("no session for UseItem")
	}

	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		log.Ctx(ctx).Warn().Str("account", session.AccountName).Msg("player not in world for UseItem")
		return nil
	}

	log.Ctx(ctx).Info().
		Int32("object_id", pkt.ObjectID).
		Int32("char_id", playerState.CharID).
		Msg("UseItem request")

	cond := usecase.PlayerCondition{IsDead: !playerState.Character.IsAlive()}

	result, err := h.inventoryUseCase.UseItem(ctx, playerState.CharID, pkt.ObjectID, cond)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("UseItem failed")
		return nil // Don't disconnect on inventory errors
	}

	// Deliver any pre-fork refusal / feedback messages (quest-item, dead,
	// reuse-remaining) regardless of Success.
	for _, m := range result.Messages {
		if err := c.Send(buildUseItemSysMsg(m)); err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("failed to send UseItem system message")
		}
	}
	// Sync the shared-group cooldown icon on the client (refusal or successful arm).
	if rs := result.ReuseSync; rs != nil {
		pkt := outclient.BuildExUseSharedGroupItem(rs.ItemID, rs.GroupID, reuseSeconds(rs.Remaining), reuseSeconds(rs.Total))
		if err := c.Send(pkt); err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("failed to send ExUseSharedGroupItem")
		}
	}

	if result.Success && len(result.ChangedItems) > 0 {
		return h.sendEquipmentUpdatePackets(ctx, c, playerState, result.ChangedItems)
	}

	return nil
}

// buildUseItemSysMsg turns a usecase.SysMsgSpec into a SystemMessage packet.
func buildUseItemSysMsg(m usecase.SysMsgSpec) []byte {
	b := outclient.NewSystemMessage(m.ID)
	if m.ItemName > 0 {
		b.AddItemName(m.ItemName)
	}
	for _, iv := range m.Ints {
		b.AddInt(iv)
	}
	return b.Build()
}

// reuseSeconds converts a reuse duration to whole seconds for ExUseSharedGroupItem
// (L2J divides the millisecond values by 1000 in the packet).
func reuseSeconds(d time.Duration) int32 {
	return int32(d / time.Second)
}

// handleRequestUnEquipItem processes RequestUnEquipItem packet (opcode 0x16)
func (h *Handler) handleRequestUnEquipItem(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt := inclient.NewRequestUnEquipItem(payload)

	session := h.getSession(c)
	if session == nil {
		return fmt.Errorf("no session for RequestUnEquipItem")
	}

	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		log.Ctx(ctx).Warn().Str("account", session.AccountName).Msg("player not in world for RequestUnEquipItem")
		return nil
	}

	log.Ctx(ctx).Info().
		Int32("slot_bitmask", pkt.SlotBitmask).
		Int32("char_id", playerState.CharID).
		Msg("RequestUnEquipItem request")

	result, err := h.inventoryUseCase.UnequipBySlot(ctx, playerState.CharID, pkt.SlotBitmask)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("UnequipBySlot failed")
		return nil
	}

	if result.Success && len(result.ChangedItems) > 0 {
		return h.sendEquipmentUpdatePackets(ctx, c, playerState, result.ChangedItems)
	}

	return nil
}

// sendEquipmentUpdatePackets sends InventoryUpdate, UserInfo, and CharInfo after equipment change
func (h *Handler) sendEquipmentUpdatePackets(ctx context.Context, c *client.ClientConn, playerState *registry.PlayerWorldState, changedItems []usecase.ChangedItem) error {
	char := playerState.Character

	// 1. Build and send InventoryUpdate with changed items
	invItems := make([]outclient.InventoryItem, 0, len(changedItems))
	for _, ci := range changedItems {
		item := ci.Item
		equipped := item.IsEquipped()

		var locSlot int32 = -1
		if equipped && item.LocData >= 0 {
			locSlot = int32(item.LocData)
		}

		bodyPart := getBodyPartBitmask(item.ItemID, item.LocData, item.Loc)
		itemType := getItemType(item.ItemID)

		invItems = append(invItems, outclient.InventoryItem{
			UpdateType:   ci.UpdateType,
			ObjectID:     item.ObjectID,
			ItemID:       item.ItemID,
			LocationSlot: locSlot,
			Count:        item.Count,
			ItemType:     itemType,
			CustomType1:  int32(item.CustomType1),
			Equipped:     equipped,
			BodyPart:     bodyPart,
			EnchantLevel: int32(item.EnchantLevel),
			CustomType2:  int32(item.CustomType2),

			AugmentationID: int32(item.AugmentationID),
			Mana:           int32(item.ManaLeft),
			TimeRemaining:  -9999,

			AttackElementType:   0,
			AttackElementPower:  0,
			DefenseElementFire:  int32(item.AttributeFire),
			DefenseElementWater: int32(item.AttributeWater),
			DefenseElementWind:  int32(item.AttributeWind),
			DefenseElementEarth: int32(item.AttributeEarth),
			DefenseElementHoly:  int32(item.AttributeHoly),
			DefenseElementDark:  int32(item.AttributeDark),
		})
	}

	invUpdate := outclient.InventoryUpdate{Items: invItems}
	if err := c.Send(outclient.BuildInventoryUpdate(invUpdate)); err != nil {
		return fmt.Errorf("failed to send InventoryUpdate: %w", err)
	}

	// 2. Update paperdoll in character model from DB (so UserInfo/CharInfo reflect new state)
	h.refreshCharacterPaperdoll(ctx, char)

	// 3. Send UserInfo to the player (updated stats + paperdoll)
	userInfoData := h.buildUserInfoPacket(char)
	if err := c.Send(userInfoData); err != nil {
		return fmt.Errorf("failed to send UserInfo: %w", err)
	}

	// 4. Broadcast CharInfo to nearby players (updated appearance)
	h.broadcastCharInfoToNearby(ctx, playerState)

	log.Ctx(ctx).Info().
		Int32("char_id", char.ID).
		Int("changed_items", len(changedItems)).
		Msg("equipment update packets sent")

	return nil
}

// refreshCharacterPaperdoll reloads paperdoll items from DB into character model
func (h *Handler) refreshCharacterPaperdoll(ctx context.Context, char *models.Character) {
	items, err := h.characterUseCase.GetCharacterAllItems(ctx, char.ID)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Int32("char_id", char.ID).Msg("failed to reload paperdoll")
		return
	}

	// Reset paperdoll
	for i := range char.PaperdollItems {
		char.PaperdollItems[i] = 0
		char.PaperdollObjectIDs[i] = 0
	}

	// Fill from equipped items
	for _, item := range items {
		if item.Loc == string(models.LocPaperdoll) && item.LocData >= 0 && item.LocData < len(char.PaperdollItems) {
			char.PaperdollItems[item.LocData] = item.ItemID
			char.PaperdollObjectIDs[item.LocData] = item.ObjectID
		}
	}
}

// broadcastCharInfoToNearby sends CharInfo to all nearby players
func (h *Handler) broadcastCharInfoToNearby(ctx context.Context, playerState *registry.PlayerWorldState) {
	nearbyPlayers := h.world.GetPlayersInRange(playerState.Position, registry.VisibilityWatchRadius)

	for _, nearby := range nearbyPlayers {
		if nearby.CharID == playerState.CharID {
			continue // Skip self
		}

		nearbyConn := h.connections.GetConnection(nearby.Character.AccountName)
		if nearbyConn == nil {
			continue
		}

		if err := h.sendPlayerSpawnToClient(ctx, nearbyConn, playerState.Character); err != nil {
			log.Ctx(ctx).Warn().Err(err).
				Int32("target_char_id", nearby.CharID).
				Msg("failed to broadcast CharInfo after equipment change")
		}
	}
}
