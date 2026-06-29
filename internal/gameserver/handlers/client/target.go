package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

// interactionDistance is the max range for NPC dialogue (L2J INTERACTION_DISTANCE).
const interactionDistance = 150

// handleAction processes the Action packet (0x1F) — player clicked on an object.
func (h *Handler) handleAction(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt, err := inclient.ParseAction(payload)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to parse Action packet")
		return c.Send(outclient.BuildActionFailed())
	}

	session := h.getSession(c)
	if session == nil {
		return c.Send(outclient.BuildActionFailed())
	}

	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		return c.Send(outclient.BuildActionFailed())
	}

	logger := log.Ctx(ctx).With().
		Int32("char_id", playerState.CharID).
		Int32("target_obj_id", pkt.ObjectID).
		Uint8("action_id", pkt.ActionID).
		Logger()

	logger.Debug().Msg("Action on object")

	// Try to find the target object (NPC or player)
	npc, targetIsNPC := h.world.GetNPC(pkt.ObjectID)
	_, targetIsPlayer := h.world.GetPlayer(pkt.ObjectID)

	if !targetIsNPC && !targetIsPlayer {
		logger.Debug().Msg("target object not found in world")
		return c.Send(outclient.BuildActionFailed())
	}

	// Check if this is a repeated click on the same target (interaction)
	if playerState.TargetID == pkt.ObjectID {
		if targetIsNPC {
			if npc.IsAttackable() {
				logger.Info().Msg("sending attack request to game loop")
				// MoveToPawn для подхода к цели шлёт game loop (handleAttackRequest),
				// чтобы offset движения и проверка дистанции удара считались по одной
				// формуле reach (L2J: движением управляет AI). См. gameloop/combat_range.go.
				// Send attack command to game loop
				h.gameLoopCmd <- gameloop.CmdAttackRequest{
					AttackerCharID: playerState.CharID,
					TargetObjectID: pkt.ObjectID,
					AttackerPos:    playerState.Position,
					AccountName:    session.AccountName,
				}
				return c.Send(outclient.BuildActionFailed())
			}
			// Distance check — L2J INTERACTION_DISTANCE = 150 units.
			// Если далеко — передаём подход в gameloop (CmdInteractRequest): он один раз
			// шлёт MoveToPawn (без ресенда на повторные клики) и открывает диалог по
			// прибытии (L2J AI_INTENTION_INTERACT). MoveToPawn здесь НЕ шлём — иначе
			// каждый клик перезапускал бы движение клиента и персонаж бы «спотыкался».
			dx := playerState.Position.X - npc.Position.X
			dy := playerState.Position.Y - npc.Position.Y
			if dx*dx+dy*dy > interactionDistance*interactionDistance {
				logger.Debug().Msg("NPC out of range — approaching to interact")
				h.gameLoopCmd <- gameloop.CmdInteractRequest{
					CharID:         playerState.CharID,
					TargetObjectID: pkt.ObjectID,
					AccountName:    session.AccountName,
				}
				return c.Send(outclient.BuildActionFailed())
			}
			// Interactive NPC — send MoveToPawn to face NPC, then dialogue HTML window
			logger.Debug().Msg("NPC interaction: opening dialogue")
			if err := c.Send(outclient.BuildMoveToPawn(
				playerState.CharID, pkt.ObjectID, 100,
				playerState.Position.X, playerState.Position.Y, playerState.Position.Z,
				npc.Position.X, npc.Position.Y, npc.Position.Z,
			)); err != nil {
				logger.Warn().Err(err).Msg("failed to send MoveToPawn")
			}
			if err := c.Send(outclient.BuildNpcHtmlMessage(pkt.ObjectID, outclient.DefaultNpcHtml)); err != nil {
				logger.Warn().Err(err).Msg("failed to send NpcHtmlMessage")
			}
			return c.Send(outclient.BuildActionFailed())
		}
		// Target is a player — PvP not implemented
		logger.Debug().Msg("player interaction not implemented")
		return c.Send(outclient.BuildActionFailed())
	}

	// First click — select target
	playerState.TargetID = pkt.ObjectID

	// Send MyTargetSelected to the clicking player
	if err := c.Send(outclient.BuildMyTargetSelected(pkt.ObjectID, 0)); err != nil {
		logger.Warn().Err(err).Msg("failed to send MyTargetSelected")
	}

	// Send StatusUpdate with HP for NPC targets (L2J sends MAX_HP + CUR_HP on setTarget)
	if targetIsNPC && npc.Template != nil {
		su := outclient.BuildStatusUpdate(pkt.ObjectID, []outclient.StatusAttribute{
			{ID: outclient.StatusMaxHP, Value: int32(npc.Template.HP)},
			{ID: outclient.StatusCurHP, Value: int32(npc.CurrentHP)},
		})
		if err := c.Send(su); err != nil {
			logger.Warn().Err(err).Msg("failed to send StatusUpdate")
		}
	}

	// Send ActionFailed to unblock the client
	return c.Send(outclient.BuildActionFailed())
}

// handleRequestTargetCancel processes the RequestTargetCancel packet (0x48).
// Sent when the player presses Escape or clicks to deselect a target.
func (h *Handler) handleRequestTargetCancel(ctx context.Context, c *client.ClientConn, payload []byte) error {
	_, err := inclient.ParseRequestTargetCancel(payload)
	if err != nil {
		log.Ctx(ctx).Debug().Err(err).Msg("failed to parse RequestTargetCancel")
		return nil
	}

	session := h.getSession(c)
	if session == nil {
		return nil
	}

	playerState, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		return nil
	}

	log.Ctx(ctx).Debug().
		Int32("char_id", playerState.CharID).
		Int32("old_target", playerState.TargetID).
		Msg("target cancelled")

	// Cancel any ongoing attack
	h.gameLoopCmd <- gameloop.CmdCancelAttack{CharID: playerState.CharID}

	// Clear player's target
	playerState.TargetID = 0

	// Send TargetUnselected to the player
	return c.Send(outclient.BuildTargetUnselected(playerState.CharID))
}
