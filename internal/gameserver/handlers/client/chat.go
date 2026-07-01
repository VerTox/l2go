package client

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

func init() { addStubRegistrator(registerChatStubs) }

// Chat text length limits (L2J Say2, verified on High Five 4 official client).
const (
	// chatMaxLen is the normal message length cap.
	chatMaxLen = 105
	// chatMaxLenWithItemLink is the cap when the text contains an item link
	// (character 0x08), which inflates the string with the link markup.
	chatMaxLenWithItemLink = 500
	// itemLinkMarker is the control character embedded around shift-clicked item links.
	itemLinkMarker = 0x08
)

// registerChatStubs регистрирует обработчики пакетов чата и социальных
// действий (High Five). Say2 реализован; остальные пока стабы.
func registerChatStubs(r *Registry) {
	// Say2 (0x49): чат (все каналы).
	r.simple[StateInGame][0x49] = packetEntry{Name: "Say2", Handle: (*Handler).handleSay2}
	// RequestSendFriendMsg (0x6b): личное сообщение другу.
	r.registerStub(StateInGame, 0x6b, "RequestSendFriendMsg")
	// AnswerCoupleAction (0xD0:0x7a): ответ на совместное действие (эмот).
	r.registerMultiStub(StateInGame, 0x7a, "AnswerCoupleAction")
	// RequestVoteNew (0xD0:0x7e): голосование.
	r.registerMultiStub(StateInGame, 0x7e, "RequestVoteNew")
}

// handleSay2 processes the Say2 packet (0x49) — a chat line on some channel.
// Validates like L2J (empty text / invalid type → ActionFailed; overlong text →
// DONT_SPAM) and routes world-aware channels through the game loop.
func (h *Handler) handleSay2(ctx context.Context, c *client.ClientConn, payload []byte) error {
	pkt := inclient.NewSay2(payload)

	session := h.getSession(c)
	if session == nil {
		return c.Send(outclient.BuildActionFailed())
	}
	player, exists := h.world.GetPlayerByAccount(session.AccountName)
	if !exists {
		return c.Send(outclient.BuildActionFailed())
	}

	// Empty text → possible packet hack. L2J also logs out the client as an
	// anti-hack measure; we intentionally do NOT (a malformed/edge client packet
	// shouldn't kick a legit player). ActionFailed is enough to unblock. (NOTES)
	if pkt.Text == "" {
		log.Ctx(ctx).Warn().Str("account", session.AccountName).Msg("Say2: empty text")
		return c.Send(outclient.BuildActionFailed())
	}

	// Invalid channel (outside client-sendable range) → ActionFailed.
	if pkt.Type < 0 || pkt.Type > outclient.ChatMaxValidType {
		log.Ctx(ctx).Warn().Int32("type", pkt.Type).Str("account", session.AccountName).Msg("Say2: invalid chat type")
		return c.Send(outclient.BuildActionFailed())
	}

	// Length guard: 105 normally, 500 when the text carries an item link (0x08).
	limit := chatMaxLen
	if strings.ContainsRune(pkt.Text, itemLinkMarker) {
		limit = chatMaxLenWithItemLink
	}
	if utf8.RuneCountInString(pkt.Text) > limit {
		return c.Send(outclient.BuildSystemMessageNoParams(outclient.SysMsgDontSpam))
	}

	charName := player.Character.Name

	switch pkt.Type {
	case outclient.ChatAll, outclient.ChatShout, outclient.ChatTell:
		h.gameLoopCmd <- gameloop.CmdChatMessage{
			SenderCharID:  player.CharID,
			SenderAccount: session.AccountName,
			ChatType:      pkt.Type,
			SenderName:    charName,
			Text:          pkt.Text,
			Target:        pkt.Target,
		}
	default:
		// Channels backed by systems not implemented yet (PARTY/CLAN/ALLIANCE/
		// TRADE/HERO_VOICE/MPCC/…). Swallow silently so the client isn't blocked.
		log.Ctx(ctx).Debug().Int32("type", pkt.Type).Str("account", session.AccountName).Msg("chat channel not implemented")
	}

	return nil
}
