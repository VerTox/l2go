package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

func init() { addStubRegistrator(registerCastHandlers) }

func registerCastHandlers(r *Registry) {
	// RequestMagicSkillUse (0x39): cast a skill. Replaces the stub (l2go-lu8).
	r.register(StateInGame, 0x39, "RequestMagicSkillUse", (*Handler).handleRequestMagicSkillUse)
}

// handleRequestMagicSkillUse forwards a cast request to the game loop, which owns
// all cast validation, timing and effects. The loop resolves the skill level from
// the caster's KnownSkills, so the handler only relays the skill id + modifiers.
func (h *Handler) handleRequestMagicSkillUse(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt, err := inclient.ParseRequestMagicSkillUse(payload)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to parse RequestMagicSkillUse")
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

	h.gameLoopCmd <- gameloop.CmdCastRequest{
		CasterCharID: playerState.CharID,
		SkillID:      pkt.MagicID,
		CtrlPressed:  pkt.CtrlPressed,
		ShiftPressed: pkt.ShiftPressed,
	}
	return nil
}
