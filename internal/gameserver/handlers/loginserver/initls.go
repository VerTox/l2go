package loginserver

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
)

func (h *Handler) handleInitLS(ctx context.Context, data []byte) error {
	log.Ctx(ctx).Info().Msg("Received InitLS packet from LoginServer")

	packet := inls.NewInitLS(data)
	if packet == nil {
		return fmt.Errorf("failed to parse InitLS packet")
	}

	// Process through use case
	publicKey, err := h.usc.HandleInitLS(ctx, packet)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed to process InitLS packet")
		return err
	}

	// Set RSA public key and send BlowFish key
	if publicKey != nil {
		h.conn.SetRSAPublicKey(publicKey)

		// Generate 16-byte BlowFish key as in Java BlowFishKeygen
		blowfishKey := make([]byte, 16)

		// First 8 bytes: random
		_, err = rand.Read(blowfishKey[:16])
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Failed to generate random BlowFish key bytes")
			return err
		}

		// Send BlowFishKey packet
		err = h.SendBlowFishKey(blowfishKey)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Failed to send BlowFishKey packet")
			return err
		}

		// Set the new Blowfish key for future communication
		h.conn.SetBlowfishKey(blowfishKey)

		log.Ctx(ctx).Info().Msg("BlowFishKey sent and encryption enabled")
	}

	log.Ctx(ctx).Debug().
		Int("rsa_key_size", len(packet.GetRSAKey())).
		Msg("InitLS packet processed successfully")

	return nil
}
