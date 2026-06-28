package loginserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/outls"
	"github.com/VerTox/l2go/internal/gameserver/transport"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// DisconnectionCallback is called when LoginServer connection is lost
type DisconnectionCallback interface {
	OnLoginServerDisconnected()
}

type Handler struct {
	conn *transport.LoginServerConnection
	usc  usecase.LoginServerCommUseCase

	mu                    sync.RWMutex
	connected             bool
	serverID              int
	serverName            string
	disconnectionCallback DisconnectionCallback
}

func New(usc usecase.LoginServerCommUseCase) *Handler {
	return &Handler{
		usc:       usc,
		connected: false,
	}
}

// SetDisconnectionCallback sets the callback for LoginServer disconnection
func (h *Handler) SetDisconnectionCallback(callback DisconnectionCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.disconnectionCallback = callback
}

// ConnectToLoginServer establishes connection to LoginServer
func (h *Handler) ConnectToLoginServer(ctx context.Context, address string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.connected {
		return fmt.Errorf("already connected to LoginServer")
	}

	log.Ctx(ctx).Info().Str("address", address).Msg("Connecting to LoginServer")

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to LoginServer: %w", err)
	}

	h.conn = transport.NewLoginServerConnection(conn)
	h.connected = true

	log.Ctx(ctx).Info().Str("address", address).Msg("Connected to LoginServer")

	// Start packet handling in background
	go h.handleConnection(ctx)

	return nil
}

// Disconnect closes the connection to LoginServer
func (h *Handler) Disconnect() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.connected || h.conn == nil {
		return nil
	}

	err := h.conn.Socket.Close()
	h.connected = false
	h.conn = nil

	return err
}

// IsConnected returns true if connected to LoginServer
func (h *Handler) IsConnected() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connected
}

// SendPacket sends a packet to LoginServer
func (h *Handler) SendPacket(data []byte) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if !h.connected || h.conn == nil {
		return fmt.Errorf("not connected to LoginServer")
	}

	return h.conn.Send(data)
}

// SendAuthRequest sends AuthRequest packet to LoginServer
func (h *Handler) SendAuthRequest(serverConfig usecase.ServerConfig, hexID []byte, subnets, hosts []string) error {
	packet := outls.NewAuthRequest(
		serverConfig.GetServerID(),
		true, // acceptAlternateID
		hexID,
		serverConfig.GetServerPort(),
		false, // reserveHost
		serverConfig.GetMaxPlayers(),
		subnets,
		hosts,
	)

	return h.SendPacket(packet.GetData())
}

// SendBlowFishKey sends BlowFishKey packet to LoginServer
func (h *Handler) SendBlowFishKey(blowfishKey []byte) error {
	if h.conn.RSAPublicKey == nil {
		return fmt.Errorf("RSA public key not set")
	}

	packet := outls.BlowFishKey{
		BlowfishKey: blowfishKey,
		PublicKey:   h.conn.RSAPublicKey,
	}
	bytes := l2pkt.BuildPacket(packet)
	return h.SendPacket(bytes)
}

// SendPlayerAuthRequest sends PlayerAuthRequest packet to LoginServer
func (h *Handler) SendPlayerAuthRequest(account string, sessionKey outls.SessionKey) error {
	packet := outls.NewPlayerAuthRequest(account, sessionKey)
	return h.SendPacket(packet.GetData())
}

// SendPlayerInGame sends PlayerInGame packet to LoginServer
func (h *Handler) SendPlayerInGame(players []string) error {
	var packet *outls.PlayerInGame
	if len(players) == 1 {
		packet = outls.NewPlayerInGame(players[0])
	} else {
		packet = outls.NewPlayerInGameMultiple(players)
	}
	return h.SendPacket(packet.GetData())
}

// SendPlayerLogout sends PlayerLogout packet to LoginServer
func (h *Handler) SendPlayerLogout(account string) error {
	packet := outls.NewPlayerLogout(account)
	return h.SendPacket(packet.GetData())
}

// SendPlayerTracert sends PlayerTracert packet to LoginServer
func (h *Handler) SendPlayerTracert(account, pcIp, hop1, hop2, hop3, hop4 string) error {
	packet := outls.NewPlayerTracert(account, pcIp, hop1, hop2, hop3, hop4)
	return h.SendPacket(packet.GetData())
}

// SendReplyCharacters sends ReplyCharacters packet to LoginServer
func (h *Handler) SendReplyCharacters(account string, charCount, charsInDel int) error {
	packet := outls.NewReplyCharacters(account, charCount, charsInDel)
	return h.SendPacket(packet.GetData())
}

// SendServerStatus sends ServerStatus packet to LoginServer
func (h *Handler) SendServerStatus(serverID, serverStatus, port, maxPlayers, serverType, minLevel, maxLevel, ageLimit int, showBrackets, pvp, testServer, showClock bool) error {
	packet := outls.NewServerStatus(serverID, serverStatus, port, maxPlayers, serverType, minLevel, maxLevel, ageLimit, showBrackets, pvp, testServer, showClock)
	return h.SendPacket(packet.GetData())
}

// SendChangePassword sends ChangePassword packet to LoginServer
func (h *Handler) SendChangePassword(accountName, characterName, oldPassword, newPassword string) error {
	packet := outls.NewChangePassword(accountName, characterName, oldPassword, newPassword)
	return h.SendPacket(packet.GetData())
}

func (h *Handler) handleConnection(ctx context.Context) {
	defer func() {
		h.mu.Lock()
		wasConnected := h.connected
		disconnectCallback := h.disconnectionCallback
		h.connected = false
		if h.conn != nil {
			h.conn.Socket.Close()
			h.conn = nil
		}
		h.mu.Unlock()
		
		// Notify GameServer about disconnection
		if wasConnected && disconnectCallback != nil {
			disconnectCallback.OnLoginServerDisconnected()
		}
	}()

	log.Ctx(ctx).Info().Msg("Starting LoginServer packet handling loop")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !h.IsConnected() {
			log.Ctx(ctx).Warn().Msg("LoginServer connection lost, stopping packet handling")
			return
		}

		// Receive and process packets
		opcode, data, err := h.conn.Receive()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Ctx(ctx).Error().Err(err).Msg("LoginServer receive error")
			} else {
				log.Ctx(ctx).Info().Msg("LoginServer connection closed, stopping packet handling")
			}
			return
		}

		if err := h.handlePacket(ctx, opcode, data); err != nil {
			log.Ctx(ctx).Error().Err(err).Uint8("opcode", opcode).Msg("LoginServer packet handling error")
			return
		}
	}
}

func (h *Handler) handlePacket(ctx context.Context, opcode byte, data []byte) error {
	log.Ctx(ctx).Debug().Int("opcode", int(opcode)).Msg("Handling LoginServer packet")

	switch opcode {
	case 0x00: // InitLS
		return h.handleInitLS(ctx, data)
	case 0x01: // LoginServerFail
		return h.handleLoginServerFail(ctx, data)
	case 0x02: // AuthResponse
		return h.handleAuthResponse(ctx, data)
	case 0x03: // PlayerAuthResponse
		return h.handlePlayerAuthResponse(ctx, data)
	case 0x04: // KickPlayer
		return h.handleKickPlayer(ctx, data)
	case 0x05: // RequestCharacters
		return h.handleRequestCharacters(ctx, data)
	case 0x06: // ChangePasswordResponse
		return h.handleChangePasswordResponse(ctx, data)
	default:
		log.Ctx(ctx).Warn().Int("opcode", int(opcode)).Msg("Unknown LoginServer packet opcode")
		return fmt.Errorf("unknown LoginServer packet opcode: 0x%02X", opcode)
	}
}
