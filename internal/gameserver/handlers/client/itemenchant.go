package client

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

func init() { addStubRegistrator(registerItemEnchantStubs) }

// registerItemEnchantStubs регистрирует обработчики пакетов заточки и атрибутов
// предметов (High Five). RequestEnchantItem (0x5f) реализован; остальные — стабы.
func registerItemEnchantStubs(r *Registry) {
	// RequestEnchantItem (0x5f): заточка предмета ранее «взведённым» свитком.
	r.register(StateInGame, 0x5f, "RequestEnchantItem", (*Handler).handleRequestEnchantItem)
	// RequestExEnchantItemAttribute (0xD0:0x35): зачаровать атрибут (стихию) предмета.
	r.registerMultiStub(StateInGame, 0x35, "RequestExEnchantItemAttribute")
	// RequestExRemoveItemAttribute (0xD0:0x23): удалить атрибут (стихию) предмета.
	r.registerMultiStub(StateInGame, 0x23, "RequestExRemoveItemAttribute")
	// RequestConfirmTargetItem (0xD0:0x26): подтвердить целевой предмет (Refinery).
	r.registerMultiStub(StateInGame, 0x26, "RequestConfirmTargetItem")
	// RequestConfirmRefinerItem (0xD0:0x27): подтвердить предмет рафинирования.
	r.registerMultiStub(StateInGame, 0x27, "RequestConfirmRefinerItem")
	// RequestConfirmGemStone (0xD0:0x28): подтвердить гемстон для рафинирования.
	r.registerMultiStub(StateInGame, 0x28, "RequestConfirmGemStone")
	// RequestRefine (0xD0:0x41): рафинировать предмет (Life Stone).
	r.registerMultiStub(StateInGame, 0x41, "RequestRefine")
	// RequestConfirmCancelItem (0xD0:0x42): подтвердить отмену (Refinery).
	r.registerMultiStub(StateInGame, 0x42, "RequestConfirmCancelItem")
	// RequestRefineCancel (0xD0:0x43): отменить рафинирование.
	r.registerMultiStub(StateInGame, 0x43, "RequestRefineCancel")
	// RequestExTryToPutEnchantTargetItem (0xD0:0x4c): поместить целевой предмет в окно заточки (HF-оконный флоу).
	r.registerMulti(StateInGame, 0x4c, "RequestExTryToPutEnchantTargetItem", (*Handler).handleRequestExTryToPutEnchantTargetItem)
	// RequestExTryToPutEnchantSupportItem (0xD0:0x4d): support-item в окне заточки. Support-бонус не реализован —
	// тихий no-op (не шлём ExPutEnchantSupportItemResult), чтобы не спамить warn'ом. См. PARKED в отчёте.
	r.registerMulti(StateInGame, 0x4d, "RequestExTryToPutEnchantSupportItem", noopStub("RequestExTryToPutEnchantSupportItem"))
	// RequestExCancelEnchantItem (0xD0:0x4e): закрыть окно заточки — сброс активного свитка.
	r.registerMulti(StateInGame, 0x4e, "RequestExCancelEnchantItem", (*Handler).handleRequestExCancelEnchantItem)
}

// handleRequestEnchantItem processes RequestEnchantItem (0x5f): the client's
// answer to a ChooseInventoryItem prompt. It performs the enchant against the
// previously-armed scroll and sends EnchantResult + InventoryUpdate + any system
// message describing the outcome.
func (h *Handler) handleRequestEnchantItem(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt := inclient.NewRequestEnchantItem(payload)

	session := h.getSession(c)
	if session == nil {
		return fmt.Errorf("no session for RequestEnchantItem")
	}
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		log.Ctx(ctx).Warn().Str("account", session.AccountName).Msg("player not in world for RequestEnchantItem")
		return nil
	}
	if h.enchantUseCase == nil {
		log.Ctx(ctx).Warn().Msg("enchant use case not wired; ignoring RequestEnchantItem")
		return nil
	}

	log.Ctx(ctx).Info().
		Int32("char_id", playerState.CharID).
		Int32("target_object_id", pkt.ObjectID).
		Int32("support_id", pkt.SupportID).
		Msg("RequestEnchantItem")

	outcome, err := h.enchantUseCase.EnchantItem(ctx, playerState.CharID, pkt.ObjectID)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("EnchantItem failed")
		return nil // never disconnect on enchant errors
	}
	if outcome == nil {
		// No armed scroll / stale prompt: nothing to do.
		return nil
	}

	// 1. EnchantResult tells the client how the attempt resolved.
	if err := c.Send(outclient.BuildEnchantResult(int32(outcome.Code), outcome.Crystal, outcome.CrystalCount)); err != nil {
		return fmt.Errorf("send EnchantResult: %w", err)
	}

	// 2. InventoryUpdate reflects scroll consumption and the item's new state.
	invItems := make([]outclient.InventoryItem, 0, 2)
	if outcome.Scroll != nil {
		updateType := int16(2) // modify
		if outcome.ScrollRemoved {
			updateType = 3 // remove
		}
		invItems = append(invItems, h.enchantInventoryItem(outcome.Scroll, updateType))
	}
	if outcome.Target != nil {
		updateType := int16(2) // modify
		if outcome.TargetDestroyed {
			updateType = 3 // remove
		}
		invItems = append(invItems, h.enchantInventoryItem(outcome.Target, updateType))
	}
	if len(invItems) > 0 {
		if err := c.Send(outclient.BuildInventoryUpdate(outclient.InventoryUpdate{Items: invItems})); err != nil {
			return fmt.Errorf("send InventoryUpdate: %w", err)
		}
	}

	// 3. Optional system message (safe/blessed failure, invalid conditions).
	if outcome.SystemMsg != 0 {
		if err := c.Send(outclient.BuildSystemMessageNoParams(outcome.SystemMsg)); err != nil {
			return fmt.Errorf("send SystemMessage: %w", err)
		}
	}

	// 4. If the enchant changed an equipped item's level, refresh the owner's
	//    UserInfo so stat bonuses update. (Best-effort; ignore on missing char.)
	if (outcome.Success || outcome.Code == usecase.EnchantCodeBlessedFail) && playerState.Character != nil {
		if err := c.Send(h.buildUserInfoPacket(playerState.Character)); err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("failed to send UserInfo after enchant")
		}
	}

	return nil
}

// handleRequestExTryToPutEnchantTargetItem processes RequestExTryToPutEnchantTargetItem
// (0xD0:0x4c): the High Five windowed enchant flow. The client, having opened the
// enchant window by double-clicking a scroll, drops a target item into it. We
// validate the target against the armed scroll and reply so the item shows up in
// the window (result=objectId) or is rejected (result=0 + system message).
func (h *Handler) handleRequestExTryToPutEnchantTargetItem(ctx context.Context, c *client.ClientConn, payload []byte) error {
	r := l2pkt.NewReader(payload)
	objectID, _ := r.ReadD()

	session := h.getSession(c)
	if session == nil {
		return fmt.Errorf("no session for RequestExTryToPutEnchantTargetItem")
	}
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		log.Ctx(ctx).Warn().Str("account", session.AccountName).Msg("player not in world for RequestExTryToPutEnchantTargetItem")
		return nil
	}
	if h.enchantUseCase == nil {
		log.Ctx(ctx).Warn().Msg("enchant use case not wired; ignoring RequestExTryToPutEnchantTargetItem")
		return nil
	}

	result, sysMsg, err := h.enchantUseCase.ValidateTarget(ctx, playerState.CharID, objectID)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("ValidateTarget failed")
		return nil // never disconnect on enchant errors
	}

	switch result {
	case usecase.PutEnchantIgnore:
		// No armed scroll / stale request: L2J returns silently.
		return nil
	case usecase.PutEnchantAccepted:
		// Target fits: it appears in the enchant window; scroll stays armed.
		if err := c.Send(outclient.BuildExPutEnchantTargetItemResult(objectID)); err != nil {
			return fmt.Errorf("send ExPutEnchantTargetItemResult: %w", err)
		}
		return nil
	default: // usecase.PutEnchantInvalid
		// Doesn't fit: notify, then reject (active state already cleared).
		if sysMsg != 0 {
			if err := c.Send(outclient.BuildSystemMessageNoParams(sysMsg)); err != nil {
				return fmt.Errorf("send SystemMessage: %w", err)
			}
		}
		if err := c.Send(outclient.BuildExPutEnchantTargetItemResult(0)); err != nil {
			return fmt.Errorf("send ExPutEnchantTargetItemResult(0): %w", err)
		}
		return nil
	}
}

// handleRequestExCancelEnchantItem processes RequestExCancelEnchantItem
// (0xD0:0x4e): the client closed the enchant window. We clear the armed scroll and
// echo EnchantResult(2) so the window dismisses, mirroring L2J.
func (h *Handler) handleRequestExCancelEnchantItem(ctx context.Context, c *client.ClientConn, _ []byte) error {
	session := h.getSession(c)
	if session == nil {
		return fmt.Errorf("no session for RequestExCancelEnchantItem")
	}
	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		log.Ctx(ctx).Warn().Str("account", session.AccountName).Msg("player not in world for RequestExCancelEnchantItem")
		return nil
	}
	if h.enchantUseCase == nil {
		return nil
	}

	h.enchantUseCase.CancelEnchant(playerState.CharID)
	if err := c.Send(outclient.BuildEnchantResult(int32(usecase.EnchantCodeError), 0, 0)); err != nil {
		return fmt.Errorf("send EnchantResult on cancel: %w", err)
	}
	return nil
}

// enchantInventoryItem builds an InventoryUpdate entry for a changed item with the
// given update type (2=modify, 3=remove).
func (h *Handler) enchantInventoryItem(item *models.CharacterItem, updateType int16) outclient.InventoryItem {
	equipped := item.IsEquipped()
	var locSlot int32 = -1
	if equipped && item.LocData >= 0 {
		locSlot = int32(item.LocData)
	}
	return outclient.InventoryItem{
		UpdateType:   updateType,
		ObjectID:     item.ObjectID,
		ItemID:       item.ItemID,
		LocationSlot: locSlot,
		Count:        item.Count,
		ItemType:     getItemType(item.ItemID),
		CustomType1:  int32(item.CustomType1),
		Equipped:     equipped,
		BodyPart:     getBodyPartBitmask(item.ItemID, item.LocData, item.Loc),
		EnchantLevel: int32(item.EnchantLevel),
		CustomType2:  int32(item.CustomType2),

		AugmentationID: int32(item.AugmentationID),
		Mana:           int32(item.ManaLeft),
		TimeRemaining:  -9999,

		DefenseElementFire:  int32(item.AttributeFire),
		DefenseElementWater: int32(item.AttributeWater),
		DefenseElementWind:  int32(item.AttributeWind),
		DefenseElementEarth: int32(item.AttributeEarth),
		DefenseElementHoly:  int32(item.AttributeHoly),
		DefenseElementDark:  int32(item.AttributeDark),
	}
}
