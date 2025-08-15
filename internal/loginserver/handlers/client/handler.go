package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/loginserver/transport"
	"github.com/VerTox/l2go/internal/loginserver/usecase"
)

type Handler struct {
	listener net.Listener

	usc                   *usecase.ClientUseCase
	gameServerUseCase     *usecase.GameServerUseCase
	gameServerCommUseCase *usecase.GameServerCommUseCase

	clients map[*transport.Client]bool
	mu      sync.RWMutex
}

func New(listener net.Listener, usc *usecase.ClientUseCase, gameServerUseCase *usecase.GameServerUseCase, gameServerCommUseCase *usecase.GameServerCommUseCase) *Handler {
	return &Handler{
		listener:              listener,
		usc:                   usc,
		gameServerUseCase:     gameServerUseCase,
		gameServerCommUseCase: gameServerCommUseCase,
		clients:               make(map[*transport.Client]bool),
	}
}

func (h *Handler) addClient(client *transport.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.clients[client]; !exists {
		h.clients[client] = true
		log.Info().Msgf("Client added: %s", client.Socket.RemoteAddr().String())
	}
}

func (h *Handler) removeClient(client *transport.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.clients[client]; exists {
		delete(h.clients, client)
		log.Info().Msgf("Client removed: %s", client.Socket.RemoteAddr().String())
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
				log.Ctx(ctx).Info().Msg("Listener closed, stopping accept loop")
				return
			}

			log.Ctx(ctx).Info().Msgf("Accept error: %v", err)
			continue
		}

		client := transport.NewClientConnection()
		client.Socket = conn

		h.addClient(client)
		go h.handleConnection(ctx, client)
	}
}

func (h *Handler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		if err := client.Socket.Close(); err != nil {
			log.Error().Msgf("Error closing client socket: %v", err)
		}
		delete(h.clients, client)
	}

	return h.listener.Close()
}

func (h *Handler) handleConnection(ctx context.Context, client *transport.Client) {
	defer h.removeClient(client)
	defer client.Socket.Close()

	if err := h.sendInitPacket(ctx, client); err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		opcode, data, err := client.Receive()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Ctx(ctx).Info().Msgf("Receive error: %v", err)
			}

			return
		}

		if err := h.handlePacket(ctx, client, opcode, data); err != nil {
			log.Ctx(ctx).Info().Msgf("Packet handling error: %v", err)

			return
		}
	}
}

func (h *Handler) handlePacket(ctx context.Context, client *transport.Client, opcode byte, data []byte) error {
	switch opcode {
	case 0x07: // AuthGameGuard
		return h.handleAuthGameGuard(ctx, client, data)
	case 0x00: // RequestAuthLogin
		return h.handleRequestAuthLogin(ctx, client, data)
	case 0x02: // RequestServerLogin
		return h.handleRequestServerLogin(ctx, client, data)
	case 0x05: // RequestServerList
		return h.handleServerList(ctx, client, data)
	default:
		log.Ctx(ctx).Warn().Int("opcode", int(opcode)).Msg("Unknown client packet opcode")
		return fmt.Errorf("unknown packet opcode: 0x%02X", opcode)
	}
}
