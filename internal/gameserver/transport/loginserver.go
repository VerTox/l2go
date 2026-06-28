package transport

import (
	"crypto/rsa"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/pkg/crypt"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LoginServerConnection represents a connection to the LoginServer
type LoginServerConnection struct {
	Socket net.Conn

	// Encryption
	blowfishKey  []byte         // Current Blowfish key for encryption
	RSAPublicKey *rsa.PublicKey // RSA public key from LoginServer
	useNewKey    bool           // false = static key, true = new key

	// Connection state
	Authenticated bool
	connected     bool
}

// NewLoginServerConnection creates a new connection to LoginServer
func NewLoginServerConnection(conn net.Conn) *LoginServerConnection {
	return &LoginServerConnection{
		Socket:        conn,
		blowfishKey:   crypt.GameServerBlowfishKey, // Start with static key
		useNewKey:     false,
		Authenticated: false,
		connected:     true,
	}
}

// SetRSAPublicKey sets the RSA public key for BlowFish key encryption
func (lsc *LoginServerConnection) SetRSAPublicKey(key *rsa.PublicKey) {
	lsc.RSAPublicKey = key
}

// SetBlowfishKey sets a new Blowfish key and enables its use
func (lsc *LoginServerConnection) SetBlowfishKey(key []byte) {
	lsc.blowfishKey = key
	lsc.useNewKey = true
}

// Send sends a packet to the LoginServer
func (lsc *LoginServerConnection) Send(data []byte) error {
	if !lsc.connected {
		return fmt.Errorf("connection closed")
	}

	data = append(data, []byte{0x00, 0x00, 0x00, 0x00}...)

	// Add Blowfish padding to make data a multiple of 8 bytes
	missing := len(data) % 8
	if missing != 0 {
		for i := missing; i < 8; i++ {
			data = append(data, byte(0x00))
		}
	}

	crypt.AppendChecksum(data)

	// Encrypt with appropriate Blowfish key
	var err error
	if lsc.useNewKey {
		// Use dynamic key after BlowFishKey packet received
		data, err = crypt.BlowfishEncrypt(data, lsc.blowfishKey)
	} else {
		// Use static GameServer key for initial packets (InitLS)
		data, err = crypt.BlowfishEncryptGameServer(data)
	}
	if err != nil {
		return fmt.Errorf("error encrypting packet: %w", err)
	}

	// Calculate the packet length
	length := uint16(len(data) + 2)

	// Put everything together
	buffer := l2pkt.NewWriter()
	buffer.WriteH(length)
	buffer.WriteB(data)

	_, err = lsc.Socket.Write(buffer.Bytes())
	if err != nil {
		return fmt.Errorf("error sending packet: %w", err)
	}

	return nil
}

// Receive receives a packet from the LoginServer
func (lsc *LoginServerConnection) Receive() (byte, []byte, error) {
	if !lsc.connected {
		return 0, nil, fmt.Errorf("connection closed")
	}

	// Read packet length (2 bytes) - read in loop until we get both bytes
	lengthBytes := make([]byte, 2)
	totalRead := 0
	for totalRead < 2 {
		n, err := lsc.Socket.Read(lengthBytes[totalRead:])
		if err != nil {
			lsc.connected = false
			return 0, nil, fmt.Errorf("failed to read packet length: %w", err)
		}
		totalRead += n
	}

	totalPacketLength := binary.LittleEndian.Uint16(lengthBytes)
	if totalPacketLength < 7 { // Minimum: length(2) + opcode(1) + checksum(4)
		return 0, nil, fmt.Errorf("packet too short: %d bytes", totalPacketLength)
	}

	// Calculate data length (total - 2 bytes for length header)
	dataLength := totalPacketLength - 2

	// Read packet data - read in loop until we get all bytes
	packetData := make([]byte, dataLength)
	totalRead2 := 0
	for totalRead2 < int(dataLength) {
		n, err := lsc.Socket.Read(packetData[totalRead2:])
		if err != nil {
			lsc.connected = false
			return 0, nil, fmt.Errorf("failed to read packet data: %w", err)
		}
		totalRead2 += n
	}

	log.Debug().
		Int("packet_len", len(packetData)).
		Str("packet_hex", fmt.Sprintf("%x", packetData[:min(32, len(packetData))])).
		Bool("use_new_key", lsc.useNewKey).
		Msg("Before decryption")

	// All packets from LoginServer are encrypted with Blowfish
	// InitLS uses static key, later packets use dynamic key after BlowFishKey exchange
	decrypted, err := crypt.BlowfishDecrypt(packetData, lsc.blowfishKey)
	if err != nil {
		return 0, nil, fmt.Errorf("blowfish decryption failed: %w", err)
	}

	// Verify checksum for all encrypted packets (including InitLS)
	if !crypt.VerifyChecksum(decrypted) {
		return 0, nil, fmt.Errorf("checksum verification failed")
	}

	// Remove checksum (last 4 bytes)
	decrypted = decrypted[:len(decrypted)-4]

	// Extract opcode and payload
	if len(decrypted) < 1 {
		return 0, nil, fmt.Errorf("decrypted packet too short")
	}

	opcode := decrypted[0]
	payload := decrypted[1:]

	log.Debug().
		Int("opcode", int(opcode)).
		Int("payload_len", len(payload)).
		Str("raw_hex", fmt.Sprintf("%x", decrypted[:min(16, len(decrypted))])).
		Msg("Received packet from LoginServer")
	return opcode, payload, nil
}

// RemoteAddr returns the remote address of the LoginServer
func (lsc *LoginServerConnection) RemoteAddr() string {
	if lsc.Socket == nil {
		return "<nil>"
	}
	return lsc.Socket.RemoteAddr().String()
}

// Close closes the connection to LoginServer
func (lsc *LoginServerConnection) Close() error {
	lsc.connected = false
	if lsc.Socket != nil {
		return lsc.Socket.Close()
	}
	return nil
}

// IsConnected returns true if the connection is active
func (lsc *LoginServerConnection) IsConnected() bool {
	return lsc.connected && lsc.Socket != nil
}
