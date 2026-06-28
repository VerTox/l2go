package usecase

import (
	"context"
	"crypto/rsa"
	"fmt"
	"math/big"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
)

// LoginServerCommUseCaseImpl implements LoginServerCommUseCase
type LoginServerCommUseCaseImpl struct {
	playerManager PlayerManager
	serverConfig  ServerConfig

	// Callback for notifying service about authentication status
	onAuthenticatedCallback func(bool)
	// Callback for sending AuthRequest after InitLS processing
	onSendAuthRequestCallback func(context.Context) error
}

// NewLoginServerCommUseCase creates a new LoginServerCommUseCase
func NewLoginServerCommUseCase(playerManager PlayerManager, serverConfig ServerConfig) LoginServerCommUseCase {
	return &LoginServerCommUseCaseImpl{
		playerManager: playerManager,
		serverConfig:  serverConfig,
	}
}

// NewLoginServerCommUseCaseWithCallback creates a new LoginServerCommUseCase with authentication callback
func NewLoginServerCommUseCaseWithCallback(playerManager PlayerManager, serverConfig ServerConfig, onAuthenticatedCallback func(bool)) LoginServerCommUseCase {
	return &LoginServerCommUseCaseImpl{
		playerManager:           playerManager,
		serverConfig:            serverConfig,
		onAuthenticatedCallback: onAuthenticatedCallback,
	}
}

// NewLoginServerCommUseCaseWithCallbacks creates a new LoginServerCommUseCase with multiple callbacks
func NewLoginServerCommUseCaseWithCallbacks(playerManager PlayerManager, serverConfig ServerConfig, onAuthenticatedCallback func(bool), onSendAuthRequestCallback func(context.Context) error) LoginServerCommUseCase {
	return &LoginServerCommUseCaseImpl{
		playerManager:             playerManager,
		serverConfig:              serverConfig,
		onAuthenticatedCallback:   onAuthenticatedCallback,
		onSendAuthRequestCallback: onSendAuthRequestCallback,
	}
}

func (uc *LoginServerCommUseCaseImpl) HandleInitLS(ctx context.Context, packet *inls.InitLS) (*rsa.PublicKey, error) {
	log.Ctx(ctx).Info().Msg("Processing InitLS packet from LoginServer")

	rsaKeyBytes := packet.GetRSAKey()
	if len(rsaKeyBytes) == 0 {
		return nil, fmt.Errorf("empty RSA key received from LoginServer")
	}

	// Parse RSA public key from modulus bytes (as sent by Java LoginServer)
	modulus := new(big.Int).SetBytes(rsaKeyBytes)

	publicKey := &rsa.PublicKey{
		N: modulus,
		E: 65537, // Commonly used exponent
	}

	log.Ctx(ctx).Info().
		Int("modulus_size", len(rsaKeyBytes)).
		Msg("RSA public key parsed successfully")

	// Trigger AuthRequest sending after successful RSA key processing
	// This will be called after BlowFishKey is sent by the handler
	if uc.onSendAuthRequestCallback != nil {
		go func() {
			// Small delay to ensure BlowFishKey is sent first
			time.Sleep(100 * time.Millisecond)
			if err := uc.onSendAuthRequestCallback(ctx); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("Failed to send AuthRequest via callback")
			}
		}()
	}

	return publicKey, nil
}

func (uc *LoginServerCommUseCaseImpl) HandleAuthResponse(ctx context.Context, packet *inls.AuthResponse) error {
	log.Ctx(ctx).Info().
		Int("server_id", packet.GetServerID()).
		Str("server_name", packet.GetServerName()).
		Msg("Processing AuthResponse packet from LoginServer")

	// Notify service about successful authentication
	if uc.onAuthenticatedCallback != nil {
		uc.onAuthenticatedCallback(true)
	}

	// TODO: Update server state with received server ID and name
	// This should store the authenticated server information
	log.Ctx(ctx).Info().Msg("GameServer authenticated with LoginServer")

	return nil
}

func (uc *LoginServerCommUseCaseImpl) HandlePlayerAuthResponse(ctx context.Context, packet *inls.PlayerAuthResponse) error {
	log.Ctx(ctx).Info().
		Str("account", packet.GetAccount()).
		Bool("authed", packet.IsAuthed()).
		Msg("Processing PlayerAuthResponse packet from LoginServer")

	if !packet.IsAuthed() {
		log.Ctx(ctx).Warn().
			Str("account", packet.GetAccount()).
			Msg("Player authentication failed")

		// TODO: Disconnect the player
		return uc.playerManager.DisconnectPlayer(ctx, packet.GetAccount())
	}

	// TODO: Complete player login process
	log.Ctx(ctx).Warn().Msg("TODO: Implement player login completion")

	return nil
}

func (uc *LoginServerCommUseCaseImpl) HandleKickPlayer(ctx context.Context, packet *inls.KickPlayer) error {
	log.Ctx(ctx).Info().
		Str("account", packet.GetAccount()).
		Msg("Processing KickPlayer packet from LoginServer")

	return uc.playerManager.DisconnectPlayer(ctx, packet.GetAccount())
}

func (uc *LoginServerCommUseCaseImpl) HandleRequestCharacters(ctx context.Context, packet *inls.RequestCharacters) error {
	log.Ctx(ctx).Info().
		Str("account", packet.GetAccount()).
		Msg("Processing RequestCharacters packet from LoginServer")

	charCount, charsInDel, err := uc.playerManager.GetCharacterCount(ctx, packet.GetAccount())
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", packet.GetAccount()).
			Msg("Failed to get character count")
		return err
	}

	log.Ctx(ctx).Debug().
		Str("account", packet.GetAccount()).
		Int("char_count", charCount).
		Int("chars_in_del", charsInDel).
		Msg("Retrieved character count")

	// TODO: Send ReplyCharacters packet back to LoginServer
	log.Ctx(ctx).Warn().Msg("TODO: Implement ReplyCharacters packet sending")

	return nil
}

func (uc *LoginServerCommUseCaseImpl) HandleLoginServerFail(ctx context.Context, packet *inls.LoginServerFail) error {
	log.Ctx(ctx).Error().
		Str("reason", packet.GetReason()).
		Msg("Processing LoginServerFail packet from LoginServer")

	// TODO: Handle server failure scenarios
	// This might involve shutting down, reconnecting, or other recovery actions
	log.Ctx(ctx).Warn().Msg("TODO: Implement login server failure handling")

	return fmt.Errorf("login server failure: %s", packet.GetReason())
}

func (uc *LoginServerCommUseCaseImpl) HandleChangePasswordResponse(ctx context.Context, packet *inls.ChangePasswordResponse) error {
	log.Ctx(ctx).Info().
		Str("account", packet.GetAccount()).
		Bool("changed", packet.HasChanged()).
		Msg("Processing ChangePasswordResponse packet from LoginServer")

	// TODO: Update player with password change result
	// This should notify the player about the password change success/failure
	log.Ctx(ctx).Warn().Msg("TODO: Implement password change result handling")

	return nil
}
