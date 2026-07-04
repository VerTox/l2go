package client

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/gameloop"
	"github.com/VerTox/l2go/internal/gameserver/models"
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

	// pendingState — запрос смены состояния соединения от обработчика, который
	// меняет состояние УСЛОВНО (напр. RequestRestart только при успешном рестарте).
	// Применяется в цикле Handle после успешной обработки пакета.
	pendingState *ConnState
}

// Handler processes game client connections.
type Handler struct {
	characterUseCase   *usecase.CharacterUseCase
	movementUseCase    usecase.MovementUseCase
	logoutUseCase      usecase.LogoutUseCase
	inventoryUseCase   *usecase.InventoryUseCase
	enchantUseCase     *usecase.EnchantUseCase
	world              *registry.WorldRegistry
	connections        *registry.ConnectionRegistry
	loginServerHandler LoginServerInterface
	gameLoopCmd        chan<- gameloop.Command
	// registry — state-aware таблица опкод→обработчик входящих пакетов.
	registry *Registry
	// skillData resolves skill templates for the SkillList passive/enchant flags.
	skillData SkillTemplateSource
	// prom records world-entry funnel metrics (l2go-5wq). nil leaves entry
	// instrumentation off; all calls are nil-safe.
	prom *gameloop.PromMetrics
	// Simple in-memory session storage (TODO: use proper session management)
	sessions map[*client.ClientConn]*ClientSession
	// sessions map is written/read from every connection goroutine; guard it.
	sessionsMu sync.Mutex
}

// SkillTemplateSource looks up a skill template by (id, level). Implemented by
// registry.SkillData; abstracted here to keep the handler transport-focused.
type SkillTemplateSource interface {
	GetSkill(skillID, level int) *models.Skill
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
		registry:           buildRegistry(),
		sessions:           make(map[*client.ClientConn]*ClientSession),
	}
}

// SetEnchantUseCase wires the enchant flow (RequestEnchantItem). Kept out of the
// New() signature so the enchant feature stays a self-contained add-on.
func (h *Handler) SetEnchantUseCase(uc *usecase.EnchantUseCase) { h.enchantUseCase = uc }

// SetSkillData wires the skill template registry used to resolve the SkillList
// passive/enchanted flags. Kept out of New() like the other add-on setters.
func (h *Handler) SetSkillData(sd SkillTemplateSource) { h.skillData = sd }

// SetPromMetrics wires the Prometheus collectors used for the world-entry funnel.
// Kept out of New() like the other add-on setters; nil leaves it off.
func (h *Handler) SetPromMetrics(pm *gameloop.PromMetrics) { h.prom = pm }

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

	// Состояние соединения: после хендшейка ждём AuthLogin.
	state := StateConnected

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

		// Для мультипакета 0xD0 вычитываем 2-байтный sub-опкод (LE) и сдвигаем payload.
		dispatchPayload := payload
		var sub uint16
		if opcode == multiPacketOpcode {
			s, rest, ok := parseSubOpcode(payload)
			if !ok {
				log.Ctx(ctx).Warn().Msg("multi-packet too short for sub-opcode")
				continue
			}
			sub, dispatchPayload = s, rest
		}

		entry, ok := h.registry.Resolve(state, opcode, sub)
		if !ok {
			if opcode == multiPacketOpcode {
				log.Ctx(ctx).Warn().
					Str("opcode", fmt.Sprintf("0xd0:0x%x", sub)).
					Uint8("state", uint8(state)).
					Msg("unknown multi-packet sub-opcode")
			} else {
				log.Ctx(ctx).Warn().
					Str("opcode", fmt.Sprintf("0x%x", opcode)).
					Uint8("state", uint8(state)).
					Msg("unknown opcode")
			}
			continue
		}

		if err := entry.Handle(h, ctx, c, dispatchPayload); err != nil {
			log.Ctx(ctx).Error().Err(err).Str("packet", entry.Name).Msg("packet handler failed")
			if entry.Fatal {
				return
			}
			continue
		}

		if entry.Transition != nil {
			state = *entry.Transition
		}
		// Условный переход состояния, запрошенный обработчиком (напр. успешный
		// RequestRestart возвращает клиента на экран выбора → StateAuthed).
		if sess := h.getSession(c); sess != nil && sess.pendingState != nil {
			state = *sess.pendingState
			sess.pendingState = nil
		}
	}
}

// Session management methods

// getSession retrieves the session for a client connection
func (h *Handler) getSession(c *client.ClientConn) *ClientSession {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()
	return h.sessions[c]
}

// setSession stores a session for a client connection
func (h *Handler) setSession(c *client.ClientConn, session *ClientSession) {
	h.sessionsMu.Lock()
	h.sessions[c] = session
	h.sessionsMu.Unlock()
	// Register connection for broadcasting
	h.connections.Register(session.AccountName, c)
}

// removeSession removes a session for a client connection and cleans up player from world
func (h *Handler) removeSession(c *client.ClientConn) {
	h.sessionsMu.Lock()
	session := h.sessions[c]
	h.sessionsMu.Unlock()
	if session != nil {
		log.Info().
			Str("account", session.AccountName).
			Msg("Client disconnected, cleaning up session and world state")

		// Unregister connection from broadcasting. Conn-aware: if a newer login
		// for this account has already replaced the registration, leave it be.
		h.connections.UnregisterIf(session.AccountName, c)

		// Remove player from world registry if they were in game
		if playerState, exists := h.world.GetPlayerByAccount(session.AccountName); exists {
			log.Info().
				Str("account", session.AccountName).
				Int32("char_id", playerState.CharID).
				Msg("Removing player from world due to disconnect")

			// Notify game loop about disconnect: it stops auto-attack, deactivates
			// regions, and despawns this player from everyone who had them in view.
			h.gameLoopCmd <- gameloop.CmdPlayerDisconnected{CharID: playerState.CharID}

			// Remove from world registry
			ctx := context.Background()
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
	h.sessionsMu.Lock()
	delete(h.sessions, c)
	h.sessionsMu.Unlock()

	log.Debug().Msg("Session cleanup completed")
}

// kickExistingSession force-disconnects any connection already logged in under
// account before a new connection for the same account is registered. This
// prevents two live sessions (and two world copies of a character) for one
// account: the new login kicks the old (L2J default behaviour).
func (h *Handler) kickExistingSession(ctx context.Context, account string, newConn *client.ClientConn) {
	old := h.connections.GetConnection(account)
	if old == nil || old == newConn {
		return
	}

	log.Ctx(ctx).Info().Str("account", account).Msg("account already connected — kicking old session")

	// Detach old session synchronously so its deferred removeSession is a no-op.
	h.sessionsMu.Lock()
	delete(h.sessions, old)
	h.sessionsMu.Unlock()

	// If the old session was in the world, tear it down like a logout.
	if playerState, exists := h.world.GetPlayerByAccount(account); exists {
		charID := playerState.CharID
		h.gameLoopCmd <- gameloop.CmdPlayerDisconnected{CharID: charID}
		h.world.RemovePlayer(ctx, charID)
		if h.logoutUseCase != nil {
			if err := h.logoutUseCase.PerformLogout(ctx, account, charID); err != nil {
				log.Ctx(ctx).Warn().Err(err).Str("account", account).Msg("kick: logout cleanup failed")
			}
		}
	}

	h.connections.Unregister(account)

	// Graceful client exit, then close the socket.
	_ = old.Send(outclient.NewLeaveWorld().GetData())
	old.Close()
}

