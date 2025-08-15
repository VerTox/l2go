package gameserver

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets/ings"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handleReplyCharacters(ctx context.Context, gs *transport.GameServer, data []byte) error {
	if !gs.Authenticated {
		return errors.New("GameServer not authenticated")
	}

	log.Ctx(ctx).Info().
		Int("server_id", gs.ID).
		Str("remote_addr", gs.RemoteAddr()).
		Msg("Received ReplyCharacters packet from GameServer")

	replyCharacters := ings.NewReplyCharacters(data)
	if replyCharacters == nil {
		return errors.New("failed to parse ReplyCharacters packet")
	}

	// Process character count through use case
	err := h.usc.HandleReplyCharacters(ctx, replyCharacters, gs.ID)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", replyCharacters.GetAccount()).
			Int("server_id", gs.ID).
			Msg("Failed to process ReplyCharacters")
		return err
	}

	log.Ctx(ctx).Info().
		Str("account", replyCharacters.GetAccount()).
		Int("character_count", replyCharacters.GetCharacterCount()).
		Int("server_id", gs.ID).
		Msg("Character count updated successfully")

	return nil
}
