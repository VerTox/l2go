package gameserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets/ings"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func (h *Handler) handleBlowFishKey(ctx context.Context, gs *transport.GameServer, data []byte) error {
	log.Ctx(ctx).Info().Str("remote_addr", gs.RemoteAddr()).Msg("Received BlowFishKey packet from GameServer")

	blowFishKey := ings.NewBlowFishKey(data)
	if blowFishKey == nil {
		return errors.New("failed to parse BlowFishKey packet")
	}

	// Decrypt the Blowfish key using our RSA private key
	decryptedKey, err := blowFishKey.DecryptKey(gs.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt BlowFish key: %w", err)
	}

	// Store the dynamic key for subsequent packets
	gs.DynamicBlowfishKey = decryptedKey

	log.Ctx(ctx).Info().
		Str("remote_addr", gs.RemoteAddr()).
		Hex("key", decryptedKey).
		Msg("Dynamic Blowfish key established")

	return nil
}
