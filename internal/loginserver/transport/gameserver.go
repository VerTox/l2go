package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/VerTox/l2go/internal/loginserver/packets"
	"github.com/VerTox/l2go/pkg/crypt"
)

type GameServer struct {
	ID                 int
	Socket             net.Conn
	PrivateKey         *rsa.PrivateKey
	DynamicBlowfishKey []byte // Dynamic key received from GameServer
	Authenticated      bool   // Whether the GameServer has completed auth
}

func NewGameServerConnection() *GameServer {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 1024)
	return &GameServer{
		PrivateKey: privateKey,
	}
}

func (gs *GameServer) RemoteAddr() string {
	if gs.Socket != nil {
		return gs.Socket.RemoteAddr().String()
	}
	return "unknown"
}

func (gs *GameServer) Receive() (opcode byte, data []byte, err error) {
	// Read the first two bytes to define the packet size
	header := make([]byte, 2)
	n, err := gs.Socket.Read(header)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return 0x00, nil, net.ErrClosed
		}
		return 0x00, nil, fmt.Errorf("error reading packet header: %w", err)
	}

	if n != 2 {
		return 0x00, nil, errors.New("incomplete packet header")
	}

	// Calculate the packet size (little-endian)
	size := int(header[0]) + int(header[1])*256

	if size < 2 {
		return 0x00, nil, errors.New("invalid packet size")
	}

	// Allocate the appropriate size for our data (size - 2 bytes used for the length)
	data = make([]byte, size-2)

	// Read the encrypted part of the packet
	n, err = gs.Socket.Read(data)
	if err != nil {
		return 0x00, nil, fmt.Errorf("error reading packet data: %w", err)
	}

	if n != size-2 {
		return 0x00, nil, fmt.Errorf("incomplete packet data: expected %d, got %d", size-2, n)
	}

	// Decrypt the packet data using appropriate Blowfish key
	if gs.DynamicBlowfishKey != nil {
		// Use dynamic key after BlowFishKey packet received
		data, err = crypt.BlowfishDecrypt(data, gs.DynamicBlowfishKey)
	} else {
		// Use static GameServer key for initial packets
		data, err = crypt.BlowfishDecryptGameServer(data)
	}
	if err != nil {
		return 0x00, nil, fmt.Errorf("error decrypting packet: %w", err)
	}

	// Verify checksum
	if !crypt.VerifyChecksum(data) {
		// Checksum verification failed - packet is corrupted or wrong key used
		// TODO: Add proper logging when logger is available in transport layer
		return 0x00, nil, errors.New("gameserver packet checksum verification failed")
	}

	// Extract the opcode
	if len(data) == 0 {
		return 0x00, nil, errors.New("empty packet data")
	}

	opcode = data[0]
	data = data[1:]

	return opcode, data, nil
}

func (gs *GameServer) Send(data []byte) error {
	// Add 4 empty bytes for the checksum
	data = append(data, []byte{0x00, 0x00, 0x00, 0x00}...)

	// Add blowfish padding to make data multiple of 8
	missing := len(data) % 8
	if missing != 0 {
		for i := missing; i < 8; i++ {
			data = append(data, byte(0x00))
		}
	}

	// Apply checksum
	crypt.AppendChecksum(data)

	// Encrypt with appropriate Blowfish key
	var err error
	if gs.DynamicBlowfishKey != nil {
		// Use dynamic key after BlowFishKey packet received
		data, err = crypt.BlowfishEncrypt(data, gs.DynamicBlowfishKey)
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
	buffer := packets.NewBuffer()
	buffer.WriteUInt16(length)
	buffer.Write(data)

	_, err = gs.Socket.Write(buffer.Bytes())
	if err != nil {
		return fmt.Errorf("error sending packet: %w", err)
	}

	return nil
}
