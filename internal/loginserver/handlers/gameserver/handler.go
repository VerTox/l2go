package gameserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/packets/outgs"
	"github.com/VerTox/l2go/internal/loginserver/transport"
	"github.com/VerTox/l2go/internal/loginserver/usecase"
)

type Handler struct {
	listener net.Listener
	usc      *usecase.GameServerCommUseCase

	gameServers map[*transport.GameServer]bool
	mu          sync.RWMutex
}

func New(listener net.Listener, usc *usecase.GameServerCommUseCase) *Handler {
	return &Handler{
		listener:    listener,
		usc:         usc,
		gameServers: make(map[*transport.GameServer]bool),
	}
}

func (h *Handler) addGameServer(gs *transport.GameServer) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.gameServers[gs] = true
	log.Info().Msgf("GameServer added: %s", gs.RemoteAddr())
}

func (h *Handler) removeGameServer(gs *transport.GameServer) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.gameServers[gs]; exists {
		delete(h.gameServers, gs)
		log.Info().Msgf("GameServer removed: %s", gs.RemoteAddr())
	}
}

func (h *Handler) ListenAndServe(ctx context.Context) {
	defer h.listener.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := h.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Ctx(ctx).Info().Msg("GameServer listener closed, stopping accept loop")
				return
			}

			log.Ctx(ctx).Error().Err(err).Msg("GameServer accept error")
			continue
		}

		gameServer := transport.NewGameServerConnection()
		gameServer.Socket = conn

		h.addGameServer(gameServer)
		go h.handleConnection(ctx, gameServer)
	}
}

func (h *Handler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for gs := range h.gameServers {
		if err := gs.Socket.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing GameServer socket")
		}
		delete(h.gameServers, gs)
	}

	return h.listener.Close()
}

func (h *Handler) handleConnection(ctx context.Context, gs *transport.GameServer) {
	defer h.removeGameServer(gs)
	defer gs.Socket.Close()

	log.Ctx(ctx).Info().Str("remote_addr", gs.RemoteAddr()).Msg("New GameServer connection")

	// Send InitLS packet immediately
	if err := h.sendInitLS(ctx, gs); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed to send InitLS packet")
		return
	}

	// Main packet handling loop
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		opcode, data, err := gs.Receive()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Ctx(ctx).Error().Err(err).Msg("GameServer receive error")
			}
			return
		}

		if err := h.handlePacket(ctx, gs, opcode, data); err != nil {
			log.Ctx(ctx).Error().Err(err).Uint8("opcode", opcode).Msg("GameServer packet handling error")
			return
		}
	}
}

func (h *Handler) sendInitLS(ctx context.Context, gs *transport.GameServer) error {
	// Get RSA public key modulus bytes
	modulusBytes := gs.PrivateKey.PublicKey.N.Bytes()

	// Java BigInteger expects positive numbers to have a leading zero if the high bit is set
	if len(modulusBytes) > 0 && modulusBytes[0]&0x80 != 0 {
		modulusBytes = append([]byte{0x00}, modulusBytes...)
	}

	initLS := outgs.NewInitLS(modulusBytes)
	err := gs.Send(initLS.GetData())
	if err != nil {
		return fmt.Errorf("failed to send InitLS packet: %w", err)
	}

	log.Ctx(ctx).Info().
		Str("remote_addr", gs.RemoteAddr()).
		Int("key_size", len(modulusBytes)).
		Msg("InitLS packet sent to GameServer")

	return nil
}

func (h *Handler) handlePacket(ctx context.Context, gs *transport.GameServer, opcode byte, data []byte) error {
	switch opcode {
	case 0x00: // BlowFishKey
		return h.handleBlowFishKey(ctx, gs, data)
	case 0x01: // AuthRequest
		return h.handleAuthRequest(ctx, gs, data)
	case 0x02: // PlayerInGame
		return h.handlePlayerInGame(ctx, gs, data)
	case 0x03: // PlayerLogout
		return h.handlePlayerLogout(ctx, gs, data)
	case 0x05: // PlayerAuthRequest
		return h.handlePlayerAuthRequest(ctx, gs, data)
	case 0x06: // ServerStatus
		return h.handleServerStatus(ctx, gs, data)
	case 0x07: // PlayerTracert
		return h.handlePlayerTracert(ctx, gs, data)
	case 0x08: // ReplyCharacters
		return h.handleReplyCharacters(ctx, gs, data)
	default:
		log.Ctx(ctx).Warn().Int("opcode", int(opcode)).Msg("Unknown GameServer packet opcode")
		return fmt.Errorf("unknown GameServer packet opcode: 0x%02X", opcode)
	}
}

// SendToServer implements GameServerPacketSender interface
func (h *Handler) SendToServer(ctx context.Context, serverID int, data []byte) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Find GameServer by ID
	for gs := range h.gameServers {
		if gs.Authenticated && gs.ID == serverID {
			err := gs.Send(data)
			if err != nil {
				return fmt.Errorf("failed to send packet to server %d: %w", serverID, err)
			}
			return nil
		}
	}

	return fmt.Errorf("authenticated GameServer %d not found", serverID)
}

// SendToAllServers implements GameServerPacketSender interface
func (h *Handler) SendToAllServers(ctx context.Context, data []byte) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sent := 0
	for gs := range h.gameServers {
		if gs.Authenticated {
			err := gs.Send(data)
			if err != nil {
				log.Ctx(ctx).Warn().Err(err).
					Int("server_id", gs.ID).
					Msg("Failed to send packet to GameServer")
				continue
			}
			sent++
		}
	}

	if sent == 0 {
		return fmt.Errorf("no authenticated GameServers found")
	}

	log.Ctx(ctx).Debug().Int("servers_sent", sent).Msg("Packet sent to all GameServers")
	return nil
}
