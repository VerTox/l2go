package usecase

import (
	"context"
	"crypto/rsa"

	"github.com/VerTox/l2go/internal/gameserver/packets/inls"
)

// LoginServerCommUseCase defines the interface for communicating with LoginServer
type LoginServerCommUseCase interface {
	// HandleInitLS processes InitLS packet and returns RSA public key
	HandleInitLS(ctx context.Context, packet *inls.InitLS) (*rsa.PublicKey, error)
	
	// HandleAuthResponse processes AuthResponse packet
	HandleAuthResponse(ctx context.Context, packet *inls.AuthResponse) error
	
	// HandlePlayerAuthResponse processes PlayerAuthResponse packet
	HandlePlayerAuthResponse(ctx context.Context, packet *inls.PlayerAuthResponse) error
	
	// HandleKickPlayer processes KickPlayer packet
	HandleKickPlayer(ctx context.Context, packet *inls.KickPlayer) error
	
	// HandleRequestCharacters processes RequestCharacters packet
	HandleRequestCharacters(ctx context.Context, packet *inls.RequestCharacters) error
	
	// HandleLoginServerFail processes LoginServerFail packet
	HandleLoginServerFail(ctx context.Context, packet *inls.LoginServerFail) error
	
	// HandleChangePasswordResponse processes ChangePasswordResponse packet
	HandleChangePasswordResponse(ctx context.Context, packet *inls.ChangePasswordResponse) error
}

// AuthResult represents the result of authentication operations
type AuthResult struct {
	Success    bool
	ServerID   int
	ServerName string
	Reason     string
}

// PlayerManager defines the interface for managing player operations
type PlayerManager interface {
	// AuthenticatePlayer validates player session and logs them in
	AuthenticatePlayer(ctx context.Context, account string, sessionKey SessionKey) (bool, error)
	
	// DisconnectPlayer forcefully disconnects a player
	DisconnectPlayer(ctx context.Context, account string) error
	
	// GetCharacterCount returns the number of characters for an account
	GetCharacterCount(ctx context.Context, account string) (int, int, error) // charCount, charsInDel, error
}

// SessionKey represents player session authentication data
type SessionKey struct {
	PlayOkID1  uint32
	PlayOkID2  uint32
	LoginOkID1 uint32
	LoginOkID2 uint32
}

// ServerConfig defines the interface for server configuration
type ServerConfig interface {
	// GetServerID returns the configured server ID
	GetServerID() int
	
	// GetServerName returns the configured server name
	GetServerName() string
	
	// GetMaxPlayers returns the maximum number of players
	GetMaxPlayers() int
	
	// GetServerPort returns the server port
	GetServerPort() int
}