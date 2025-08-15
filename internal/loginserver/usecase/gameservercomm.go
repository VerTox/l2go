package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/events"
	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/packets/ings"
	"github.com/VerTox/l2go/internal/loginserver/packets/outgs"
	"github.com/VerTox/l2go/internal/loginserver/registry"
)

// GameServerCommUseCase handles GameServer communication logic
type GameServerCommUseCase struct {
	gameServerRegistry     registry.GameServerRegistry
	sessionUseCase         *SessionUseCase
	characterCountRegistry registry.CharacterCountRegistry
	eventBus               events.EventBus
}

type GameServerCommParams struct {
	GameServerRegistry     registry.GameServerRegistry
	SessionUseCase         *SessionUseCase
	CharacterCountRegistry registry.CharacterCountRegistry
	EventBus               events.EventBus
}

func NewGameServerCommUseCase(params GameServerCommParams) *GameServerCommUseCase {
	return &GameServerCommUseCase{
		gameServerRegistry:     params.GameServerRegistry,
		sessionUseCase:         params.SessionUseCase,
		characterCountRegistry: params.CharacterCountRegistry,
		eventBus:               params.EventBus,
	}
}

// HandleAuthRequest processes GameServer authentication request
func (uc *GameServerCommUseCase) HandleAuthRequest(ctx context.Context, authReq *ings.AuthRequest) (*AuthResult, error) {
	serverID := authReq.GetID()
	maxPlayers := authReq.GetMaxPlayers()
	port := authReq.GetPort()

	log.Ctx(ctx).Info().
		Int("server_id", serverID).
		Int("max_players", maxPlayers).
		Int("port", port).
		Msg("GameServer authentication request")

	// Generate server name based on ID (matching L2J convention)
	serverName := uc.generateServerName(serverID)

	// Create GameServer info for registry
	serverInfo := &models.GameServerInfo{
		ID:             serverID,
		Name:           serverName,
		Port:           port,
		Status:         models.ServerStatusOnline,
		CurrentPlayers: 0,
		MaxPlayers:     maxPlayers,
		PvP:            true, // Default
		AgeLimit:       0,    // No age limit
		ServerType:     1,    // Normal server
		ShowBrackets:   false,
	}

	// Add server addresses from AuthRequest hosts
	hosts := authReq.GetHosts()
	log.Ctx(ctx).Info().
		Int("server_id", serverID).
		Int("hosts_count", len(hosts)).
		Msg("Processing GameServer hosts")

	// Process hosts in pairs: [subnet, ip, subnet, ip, ...]
	for i := 0; i+1 < len(hosts); i += 2 {
		subnet := hosts[i]
		serverAddr := hosts[i+1]

		err := serverInfo.AddServerAddress(subnet, serverAddr)
		if err != nil {
			log.Ctx(ctx).Warn().
				Err(err).
				Str("subnet", subnet).
				Str("server_addr", serverAddr).
				Msg("Failed to add server address, skipping")
			continue
		}

		log.Ctx(ctx).Debug().
			Str("subnet", subnet).
			Str("server_addr", serverAddr).
			Msg("Added server address mapping")
	}

	// Register or update server in registry
	err := uc.gameServerRegistry.Register(ctx, serverInfo)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Int("server_id", serverID).Msg("Failed to register GameServer")
		return &AuthResult{Success: false, Reason: "Registration failed"}, err
	}

	log.Ctx(ctx).Info().
		Int("server_id", serverID).
		Str("server_name", serverName).
		Msg("GameServer registered successfully")

	return &AuthResult{
		Success:    true,
		ServerID:   serverID,
		ServerName: serverName,
	}, nil
}

// HandlePlayerAuthRequest processes player authentication from GameServer
func (uc *GameServerCommUseCase) HandlePlayerAuthRequest(ctx context.Context, authReq *ings.PlayerAuthRequest) (*PlayerAuthResult, error) {
	account := authReq.GetAccount()
	requestedKey := authReq.GetSessionKey()

	log.Ctx(ctx).Info().
		Str("account", account).
		Uint32("login_key1", requestedKey.LoginKey1).
		Uint32("login_key2", requestedKey.LoginKey2).
		Uint32("play_key1", requestedKey.PlayKey1).
		Uint32("play_key2", requestedKey.PlayKey2).
		Msg("Player authentication request from GameServer")

	// Validate session key
	valid, err := uc.sessionUseCase.ValidateSessionKey(ctx, account, requestedKey)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("account", account).Msg("Error validating session key")
		return &PlayerAuthResult{
			Account: account,
			Success: false,
			Reason:  "Session validation error",
		}, err
	}

	if !valid {
		log.Ctx(ctx).Warn().Str("account", account).Msg("Player authentication failed - invalid session key")
		return &PlayerAuthResult{
			Account: account,
			Success: false,
			Reason:  "Invalid session key",
		}, nil
	}

	// Remove the session key as it's now consumed (normal part of auth flow)
	err = uc.sessionUseCase.RemoveSessionKey(ctx, account)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("account", account).Msg("Failed to remove session key during authentication")
		// Don't fail the auth for this
	} else {
		log.Ctx(ctx).Debug().Str("account", account).Msg("Session key consumed and removed after successful authentication")
	}

	log.Ctx(ctx).Info().Str("account", account).Msg("Player authentication successful")

	return &PlayerAuthResult{
		Account: account,
		Success: true,
		Reason:  "Authentication successful",
	}, nil
}

// HandlePlayerLogout processes player logout notification
func (uc *GameServerCommUseCase) HandlePlayerLogout(ctx context.Context, account string, serverID int) error {
	log.Ctx(ctx).Info().
		Str("account", account).
		Int("server_id", serverID).
		Msg("Player logout notification from GameServer")

	// Clean up any remaining session keys (defensive cleanup)
	// Note: Session keys are normally removed during PlayerAuthRequest,
	// so this is expected to fail in most normal cases
	err := uc.sessionUseCase.RemoveSessionKey(ctx, account)
	if err != nil {
		// This is expected behavior - session key was already removed during authentication
		log.Ctx(ctx).Debug().Err(err).
			Str("account", account).
			Msg("Session key already removed (normal - removed during authentication)")
	} else {
		// This means session key was still present - player logged out before proper authentication
		log.Ctx(ctx).Info().
			Str("account", account).
			Msg("Session key cleaned up on logout (player disconnected before completing authentication)")
	}

	return nil
}

// UpdateServerStatus updates a GameServer's status
func (uc *GameServerCommUseCase) UpdateServerStatus(ctx context.Context, serverID int, status models.ServerStatus) error {
	return uc.gameServerRegistry.UpdateStatus(ctx, serverID, status)
}

// UpdateServerPlayerCount updates a GameServer's player count
func (uc *GameServerCommUseCase) UpdateServerPlayerCount(ctx context.Context, serverID int, current, max int) error {
	return uc.gameServerRegistry.UpdatePlayerCount(ctx, serverID, current, max)
}

// generateServerName generates a server name based on server ID (L2J convention)
func (uc *GameServerCommUseCase) generateServerName(serverID int) string {
	serverNames := map[int]string{
		1: "Bartz", 2: "Sieghardt", 3: "Kain", 4: "Lionna", 5: "Erica",
		6: "Gustin", 7: "Devianne", 8: "Hindemith", 9: "Teon (EURO)",
		10: "Franz (EURO)", 11: "Luna (EURO)", 12: "Sayha", 13: "Aria",
		14: "Phoenix", 15: "Chronos", 16: "Naia (EURO)", 17: "Elhwynna",
	}

	if name, exists := serverNames[serverID]; exists {
		return name
	}

	return fmt.Sprintf("Server_%d", serverID)
}

// AuthResult represents the result of GameServer authentication
type AuthResult struct {
	Success    bool
	ServerID   int
	ServerName string
	Reason     string
}

// PlayerAuthResult represents the result of player authentication
type PlayerAuthResult struct {
	Account string
	Success bool
	Reason  string
}

// HandleReplyCharacters processes character count reply from GameServer
func (uc *GameServerCommUseCase) HandleReplyCharacters(ctx context.Context, reply *ings.ReplyCharacters, serverID int) error {
	account := reply.GetAccount()
	characterCount := reply.GetCharacterCount()

	log.Ctx(ctx).Info().
		Str("account", account).
		Int("character_count", characterCount).
		Int("server_id", serverID).
		Msg("Processing character count reply")

	// Store character count in registry
	err := uc.characterCountRegistry.SetCharacterCount(ctx, account, serverID, characterCount)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).
			Str("account", account).
			Int("server_id", serverID).
			Msg("Failed to store character count")
		return err
	}

	return nil
}

// GetCharacterCounts returns character counts for an account across all servers
func (uc *GameServerCommUseCase) GetCharacterCounts(ctx context.Context, account string) map[int]int {
	return uc.characterCountRegistry.GetCharacterCounts(ctx, account)
}

// ClearServerCharacterCounts clears all character count data for a server (when server disconnects)
func (uc *GameServerCommUseCase) ClearServerCharacterCounts(ctx context.Context, serverID int) error {
	return uc.characterCountRegistry.ClearServer(ctx, serverID)
}

// RequestCharacterCountsFromAllServers sends RequestCharacters packets to all authenticated GameServers
func (uc *GameServerCommUseCase) RequestCharacterCountsFromAllServers(ctx context.Context, account string) error {
	servers, err := uc.gameServerRegistry.GetAll(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed to get server list")
		return err
	}

	log.Ctx(ctx).Info().
		Str("account", account).
		Int("server_count", len(servers)).
		Msg("Requesting character counts from all servers")

	// Create RequestCharacters packet
	requestCharacters := outgs.NewRequestCharacters(account)
	packetData := requestCharacters.GetData()

	// Send via EventBus
	if uc.eventBus == nil {
		log.Ctx(ctx).Error().Msg("EventBus not available")
		return errors.New("EventBus not available")
	}

	return uc.sendViaEventBus(ctx, account, servers, packetData)
}

// sendViaEventBus sends packets using the EventBus pattern
func (uc *GameServerCommUseCase) sendViaEventBus(ctx context.Context, account string, servers []*models.GameServerInfo, packetData []byte) error {
	for _, server := range servers {
		if server.IsOnline() {
			event := &events.SendPacketEvent{
				ServerID: server.ID,
				Data:     packetData,
			}

			err := uc.eventBus.Publish(ctx, event)
			if err != nil {
				log.Ctx(ctx).Warn().Err(err).
					Str("account", account).
					Int("server_id", server.ID).
					Msg("Failed to publish SendPacket event")
				continue
			}

			log.Ctx(ctx).Debug().
				Str("account", account).
				Int("server_id", server.ID).
				Msg("SendPacket event published successfully")
		}
	}
	return nil
}
