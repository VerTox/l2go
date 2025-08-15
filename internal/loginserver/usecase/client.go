package usecase

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/packets/inclient"
	"github.com/VerTox/l2go/internal/loginserver/repo"
)

const (
	ACCESS_LEVEL_BANNED = -1
	ACCESS_LEVEL_PLAYER = 0
	ACCESS_LEVEL_ADMIN  = 1
)

var (
	ErrAccountBanned   = errors.New("account is banned")
	ErrAccountNotFound = errors.New("account not found")
)

type ClientUseCase struct {
	repo           repo.Repository
	sessionUseCase *SessionUseCase

	config config
}

type config struct {
	autoCreate bool
}

type Params struct {
	Repo              repo.Repository
	AutoCreateAccount bool
	SessionUseCase    *SessionUseCase
}

func NewClientUseCase(p Params) *ClientUseCase {
	return &ClientUseCase{
		repo:           p.Repo,
		sessionUseCase: p.SessionUseCase,
		config: config{
			autoCreate: p.AutoCreateAccount,
		},
	}
}

func (uc *ClientUseCase) HandleAuthLogin(ctx context.Context, request *inclient.RequestAuthLogin) (*models.Account, error) {
	log.Ctx(ctx).Info().Msgf("User %s is trying to login", request.Username)

	// Try to find existing account
	log.Ctx(ctx).Debug().Msgf("Looking up account %s", request.Username)
	account, err := uc.repo.Account.GetByUsername(ctx, request.Username)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("Database error while looking up account %s: %v", request.Username, err)
		return nil, ErrAccountNotFound
	}

	if account != nil {
		if account.AccessLevel == ACCESS_LEVEL_BANNED {
			return nil, ErrAccountBanned
		}

		err = bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(request.Password))

		if err != nil {
			return nil, err
		}

		return account, nil
	}

	// Account doesn't exist
	if uc.config.autoCreate == true {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), 10)
		if err != nil {
			log.Ctx(ctx).Error().Msg("An error occurred while trying to generate the password")
			return nil, err
		}

		newAccount := &models.Account{
			Username:    request.Username,
			Password:    string(hashedPassword),
			AccessLevel: ACCESS_LEVEL_PLAYER,
		}

		err = uc.repo.Account.Create(ctx, newAccount)
		if err != nil {
			log.Ctx(ctx).Error().Msgf("Couldn't create an account for the user %s: %v", request.Username, err)

			return nil, err
		}

		log.Ctx(ctx).Info().Msgf("Account successfully created for the user %s", request.Username)

		return newAccount, nil

	}

	return nil, ErrAccountNotFound
}

// RequestServerLoginResult represents the result of a server login request
type RequestServerLoginResult struct {
	Success  bool
	PlayKey1 uint32
	PlayKey2 uint32
	Reason   string
}

// HandleRequestServerLogin handles a request to connect to a specific game server
func (uc *ClientUseCase) HandleRequestServerLogin(ctx context.Context, request *inclient.RequestServerLogin, account *models.Account, loginOkID1, loginOkID2 uint32) (*RequestServerLoginResult, error) {
	if account == nil {
		return &RequestServerLoginResult{
			Success: false,
			Reason:  "Account not authenticated",
		}, nil
	}

	// Check if user has access to the requested server
	// For now, we'll allow access to all servers for authenticated users
	serverID := int(request.ServerID)

	// Create session key for this login attempt
	sessionKey, err := uc.sessionUseCase.CreateSessionKey(ctx, account.Username, serverID, loginOkID1, loginOkID2)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("account", account.Username).Int("server_id", serverID).Msg("Failed to create session key")
		return &RequestServerLoginResult{
			Success: false,
			Reason:  "Failed to create session key",
		}, err
	}

	log.Ctx(ctx).Info().
		Str("account", account.Username).
		Int("server_id", serverID).
		Msg("Player authorized to connect to game server")

	return &RequestServerLoginResult{
		Success:  true,
		PlayKey1: sessionKey.PlayKey1,
		PlayKey2: sessionKey.PlayKey2,
	}, nil
}
