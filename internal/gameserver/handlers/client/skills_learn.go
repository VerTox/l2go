package client

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

func init() { addStubRegistrator(registerSkillLearnHandlers) }

// registerSkillLearnHandlers wires the NPC skill-learning flow (l2go-hv9),
// replacing the stubs for bypass + acquire-skill packets.
func registerSkillLearnHandlers(r *Registry) {
	r.register(StateInGame, 0x23, "RequestBypassToServer", (*Handler).handleRequestBypassToServer)
	r.register(StateInGame, 0x73, "RequestAcquireSkillInfo", (*Handler).handleRequestAcquireSkillInfo)
	r.register(StateInGame, 0x7c, "RequestAcquireSkill", (*Handler).handleRequestAcquireSkill)
}

// learnSkillsBypass is the bypass token our trainer HTML uses to open the learn window.
const learnSkillsBypass = "learn_skills"

func (h *Handler) handleRequestBypassToServer(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt, err := inclient.ParseRequestBypass(payload)
	if err != nil {
		log.Ctx(ctx).Debug().Err(err).Msg("failed to parse RequestBypassToServer")
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

	if pkt.Command == learnSkillsBypass {
		// Trainer is the currently-targeted NPC.
		h.gameLoopCmd <- gameloop.CmdOpenSkillLearn{
			CharID:   playerState.CharID,
			NpcObjID: playerState.TargetID,
		}
		return nil
	}
	log.Ctx(ctx).Debug().Str("cmd", pkt.Command).Msg("unhandled bypass command")
	return nil
}

func (h *Handler) handleRequestAcquireSkillInfo(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt, err := inclient.ParseRequestAcquireSkill(payload)
	if err != nil {
		log.Ctx(ctx).Debug().Err(err).Msg("failed to parse RequestAcquireSkillInfo")
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
	h.gameLoopCmd <- gameloop.CmdSkillLearnInfo{
		CharID:  playerState.CharID,
		SkillID: pkt.SkillID,
		Level:   pkt.Level,
	}
	return nil
}

func (h *Handler) handleRequestAcquireSkill(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt, err := inclient.ParseRequestAcquireSkill(payload)
	if err != nil {
		log.Ctx(ctx).Debug().Err(err).Msg("failed to parse RequestAcquireSkill")
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
	h.gameLoopCmd <- gameloop.CmdLearnSkill{
		CharID:   playerState.CharID,
		NpcObjID: playerState.TargetID,
		SkillID:  pkt.SkillID,
		Level:    pkt.Level,
	}
	return nil
}
