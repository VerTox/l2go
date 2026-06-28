package client

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/packets/inclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/packets/outls"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

// ClientSession represents a client session
type ClientSession struct {
	AccountName string
	SessionID   uint32
	LoginKeys   [2]uint32 // LoginOkID1, LoginOkID2
	PlayKeys    [2]uint32 // PlayOkID1, PlayOkID2
}

// Handler processes game client connections.
type Handler struct {
	characterUseCase   *usecase.CharacterUseCase
	movementUseCase    usecase.MovementUseCase
	logoutUseCase      usecase.LogoutUseCase
	inventoryUseCase   *usecase.InventoryUseCase
	world              *registry.WorldRegistry
	connections        *registry.ConnectionRegistry
	loginServerHandler LoginServerInterface
	gameLoopCmd        chan<- gameloop.Command
	// Simple in-memory session storage (TODO: use proper session management)
	sessions map[*client.ClientConn]*ClientSession
}

// LoginServerInterface provides methods to communicate with LoginServer
type LoginServerInterface interface {
	SendPlayerAuthRequest(account string, sessionKey outls.SessionKey) error
}

func New(characterUseCase *usecase.CharacterUseCase, movementUseCase usecase.MovementUseCase, logoutUseCase usecase.LogoutUseCase, inventoryUseCase *usecase.InventoryUseCase, world *registry.WorldRegistry, connections *registry.ConnectionRegistry, loginServerHandler LoginServerInterface, gameLoopCmd chan<- gameloop.Command) *Handler {
	return &Handler{
		characterUseCase:   characterUseCase,
		movementUseCase:    movementUseCase,
		logoutUseCase:      logoutUseCase,
		inventoryUseCase:   inventoryUseCase,
		world:              world,
		connections:        connections,
		loginServerHandler: loginServerHandler,
		gameLoopCmd:        gameLoopCmd,
		sessions:           make(map[*client.ClientConn]*ClientSession),
	}
}

// Handle processes incoming client packets
func (h *Handler) Handle(ctx context.Context, c *client.ClientConn) {
	// Cleanup session on disconnect
	defer h.removeSession(c)
	// Step 1: read ProtocolVersion (unencrypted)
	opcode, payload, err := c.Receive()
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("read ProtocolVersion failed")
		return
	}
	if opcode != 0x0e {
		log.Ctx(ctx).Warn().Uint8("opcode", opcode).Msg("unexpected first opcode; expected ProtocolVersion")
	}
	_ = inclient.NewProtocolVersion(payload) // parsed value not used yet

	// Step 2: send KeyPacket (unencrypted) and enable 16-byte GameCrypt
	var key16 [16]byte
	// first 8 bytes random
	if _, err := rand.Read(key16[:8]); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("random key generation failed")
		return
	}
	// last 8 bytes are static (per L2J BlowFishKeygen)
	copy(key16[8:], []byte{0xc8, 0x27, 0x93, 0x01, 0xa1, 0x6c, 0x31, 0x97})

	// Включаем крипт (установим ключ); первый send станет триггером включения внутри шифра
	if err := c.EnableCrypt(key16[:]); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("enable GameCrypt failed")
		return
	}
	keyPkt := outclient.NewKeyPacket(key16[:], true, 1, 0)
	if err := c.Send(keyPkt); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("send KeyPacket failed")
		return
	}

	log.Ctx(ctx).Info().Msg("Client handshake complete (GameCrypt enabled)")

	for {
		select {
		case <-ctx.Done():
			log.Ctx(ctx).Info().Msg("client handler context done")
			return
		default:
		}

		// Read packet from client
		opcode, payload, err := c.Receive()
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to read client packet")
			return
		}

		log.Ctx(ctx).Debug().
			Str("opcode", fmt.Sprintf("0x%x", opcode)).
			Msg("client packet")

		// Route to appropriate handler based on opcode
		switch opcode {
		case 0x2b: // AuthLogin
			if err := h.handleAuthLogin(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle AuthLogin failed")
				return
			}
		case 0x12: // CharacterSelect
			if err := h.handleCharacterSelect(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle CharacterSelect failed")
				return
			}
		case 0x13: // NewCharacter (request templates)
			if err := h.handleNewCharacter(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle NewCharacter failed")
				return
			}
		case 0x0c: // CharacterCreate
			if err := h.handleCharacterCreate(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle CharacterCreate failed")
				return
			}
		case 0x0d: // CharacterDelete
			if err := h.handleCharacterDelete(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle CharacterDelete failed")
				return
			}
		case 0x11: // EnterWorld
			if err := h.handleEnterWorld(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle EnterWorld failed")
				return
			}
		case 0x1f: // Action (click on NPC/player/object)
			if err := h.handleAction(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle Action failed")
			}
		case 0x48: // RequestTargetCancel (Escape / deselect target)
			if err := h.handleRequestTargetCancel(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle RequestTargetCancel failed")
			}
		case 0x0f: // MoveBackwardToLocation
			if err := h.handleMoveBackwardToLocation(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle MoveBackwardToLocation failed")
			}
		case 0x59: // ValidatePosition
			if err := h.handleValidatePosition(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle ValidatePosition failed")
			}
		case 0x47: // CannotMoveAnymore
			if err := h.handleCannotMoveAnymore(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle CannotMoveAnymore failed")
			}
		case 0x52: // MoveWithDelta (not implemented in L2J, stub for compatibility)
			log.Ctx(ctx).Debug().Msg("MoveWithDelta packet received (not implemented, ignoring)")
			continue
		case 0x56: // RequestActionUse (Walk/Run toggle, etc.)
			if err := h.handleRequestActionUse(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle RequestActionUse failed")
			}
		case 0x00: // Logout
			if err := h.handleLogout(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle Logout failed")
			}
		case 0x57: // RequestRestart
			if err := h.handleRequestRestart(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle RequestRestart failed")
			}
		case 0xd0: // Multi-packet opcode (requires sub-opcode)
			if err := h.handleMultiPacket(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle multi-packet failed")
			}
		case 0x14:
			if err := h.handleRequestItemList(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle RequestItemList failed")
			}
		case 0x19: // UseItem (equip/unequip by double-click)
			if err := h.handleUseItem(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle UseItem failed")
			}
		case 0x16: // RequestUnEquipItem (drag off paperdoll)
			if err := h.handleRequestUnEquipItem(ctx, c, payload); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("handle RequestUnEquipItem failed")
			}
		case 0xa6: //RequestSkillCoolTime
			continue
		default:
			log.Ctx(ctx).Warn().Msgf("unknown opcode 0x%x", opcode)
		}
	}
}

// Session management methods

// getSession retrieves the session for a client connection
func (h *Handler) getSession(c *client.ClientConn) *ClientSession {
	return h.sessions[c]
}

// setSession stores a session for a client connection
func (h *Handler) setSession(c *client.ClientConn, session *ClientSession) {
	h.sessions[c] = session
	// Register connection for broadcasting
	h.connections.Register(session.AccountName, c)
}

// removeSession removes a session for a client connection and cleans up player from world
func (h *Handler) removeSession(c *client.ClientConn) {
	session := h.sessions[c]
	if session != nil {
		log.Info().
			Str("account", session.AccountName).
			Msg("Client disconnected, cleaning up session and world state")

		// Unregister connection from broadcasting
		h.connections.Unregister(session.AccountName)

		// Remove player from world registry if they were in game
		if playerState, exists := h.world.GetPlayerByAccount(session.AccountName); exists {
			log.Info().
				Str("account", session.AccountName).
				Int32("char_id", playerState.CharID).
				Msg("Removing player from world due to disconnect")

			// Notify game loop about disconnect (stops auto-attack, deactivates regions)
			h.gameLoopCmd <- gameloop.CmdPlayerDisconnected{CharID: playerState.CharID}

			// Send DeleteObject to nearby players first (before removing from world)
			ctx := context.Background()
			h.broadcastPlayerDespawn(ctx, playerState)

			// Remove from world registry
			h.world.RemovePlayer(ctx, playerState.CharID)

			// If we have logout use case, perform cleanup
			if h.logoutUseCase != nil {
				ctx := context.Background() // Use background context for cleanup
				if err := h.logoutUseCase.PerformLogout(ctx, session.AccountName, playerState.CharID); err != nil {
					log.Warn().
						Err(err).
						Str("account", session.AccountName).
						Msg("Failed to perform logout cleanup on disconnect")
				} else {
					log.Debug().
						Str("account", session.AccountName).
						Msg("Logout cleanup completed on disconnect")
				}
			}
		}
	}

	// Remove session from memory
	delete(h.sessions, c)
	
	log.Debug().Msg("Session cleanup completed")
}

// broadcastPlayerDespawn sends DeleteObject packet to nearby players when a player leaves
func (h *Handler) broadcastPlayerDespawn(ctx context.Context, playerState *registry.PlayerWorldState) {
	logger := log.Ctx(ctx).With().
		Int32("leaving_char_id", playerState.CharID).
		Str("leaving_name", playerState.Character.Name).
		Logger()

	// Get nearby players who can see this player
	nearbyPlayers := h.world.GetPlayersInRange(playerState.Position, 1500)
	if len(nearbyPlayers) == 0 {
		logger.Debug().Msg("no nearby players to notify of despawn")
		return
	}

	// Create DeleteObject packet
	deletePacket := outclient.NewDeleteObject(playerState.CharID)
	packetData := deletePacket.GetData()
	
	broadcastCount := 0
	
	// Send to all nearby players
	for _, nearby := range nearbyPlayers {
		if nearby.CharID != playerState.CharID { // Don't send to the leaving player
			nearbyConn := h.connections.GetConnection(nearby.Character.AccountName)
			if nearbyConn != nil {
				if err := nearbyConn.Send(packetData); err != nil {
					logger.Warn().Err(err).
						Str("nearby_account", nearby.Character.AccountName).
						Msg("failed to send DeleteObject to nearby player")
				} else {
					broadcastCount++
					logger.Debug().
						Str("nearby_account", nearby.Character.AccountName).
						Msg("DeleteObject sent to nearby player")
				}
			}
		}
	}
	
	logger.Info().
		Int("broadcasts_sent", broadcastCount).
		Int("nearby_players", len(nearbyPlayers)-1). // -1 for the leaving player
		Msg("player despawn broadcasted to nearby players")
}
