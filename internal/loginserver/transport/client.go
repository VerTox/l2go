package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	mathrand "math/rand"
	"net"

	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/packets"
	"github.com/VerTox/l2go/pkg/crypt"
)

type Client struct {
	SessionID   []byte
	Socket      net.Conn
	BlowfishKey []byte          // Dynamic Blowfish key for this inclient
	PrivateKey  *rsa.PrivateKey // RSA private key for this inclient
	LoginOkID1  uint32          // LoginOk ID 1 for session key
	LoginOkID2  uint32          // LoginOk ID 2 for session key

	Account     *models.Account
	AccessLevel int // Player access level (0 = normal player, >0 = GM)
	LastServer  int // Last connected server ID
}

func NewClientConnection() *Client {
	id := make([]byte, 16)
	_, err := rand.Read(id)

	if err != nil {
		return nil
	}

	// Generate dynamic Blowfish key for this inclient
	blowfishKey := make([]byte, 16)
	rand.Read(blowfishKey)

	privateKey, _ := rsa.GenerateKey(rand.Reader, 1024)

	return &Client{
		SessionID:   id,
		BlowfishKey: blowfishKey,
		PrivateKey:  privateKey,
	}
}

func (c *Client) Receive() (opcode byte, data []byte, e error) {
	// Read the first two bytes to define the packet size
	header := make([]byte, 2)
	n, err := c.Socket.Read(header)

	if n != 2 || err != nil {
		if errors.Is(err, io.EOF) {
			// Connection closed by client (normal)
			return 0x00, nil, net.ErrClosed
		}
		// TODO: Add proper logging when logger is available in transport layer
		return 0x00, nil, errors.New("an error occured while reading the packet header")
	}

	// Calculate the packet size
	size := 0
	size = size + int(header[0])
	size = size + int(header[1])*256

	// Allocate the appropriate size for our data (size - 2 bytes used for the length
	data = make([]byte, size-2)

	// Read the encrypted part of the packet
	n, err = c.Socket.Read(data)

	if n != size-2 || err != nil {
		// TODO: Add proper logging when logger is available in transport layer
		return 0x00, nil, errors.New("an error occured while reading the packet data")
	}

	// Decrypt the packet data using dynamic key
	// According to Java LoginCrypt, ALL incoming packets use the dynamic key
	// The static key is only used for the first OUTGOING packet (server to inclient)
	decryptionKey := c.BlowfishKey

	data, err = crypt.BlowfishDecrypt(data, decryptionKey)

	if err != nil {
		return 0x00, nil, errors.New("An error occured while decrypting the packet data.")
	}

	// Verify checksum using standard server packet verification (as per Java LoginCrypt)
	checksumValid := crypt.VerifyChecksum(data)
	if !checksumValid {
		// Try client-specific checksum verification for packets with padding
		checksumValid = crypt.VerifyChecksumClient(data)
	}

	if !checksumValid {
		// Checksum verification failed - packet is corrupted or wrong key used
		// TODO: Add proper logging when logger is available in transport layer
		return 0x00, nil, errors.New("packet checksum verification failed")
	}

	// Extract the op code
	opcode = data[0]
	data = data[1:]
	e = nil
	return
}

func (c *Client) Send(data []byte, params ...bool) error {
	var doChecksum, doBlowfish bool = true, true

	// Should we skip the checksum?
	if len(params) >= 1 && params[0] == false {
		doChecksum = false
	}

	// Should we skip the blowfish encryption?
	if len(params) >= 2 && params[1] == false {
		doBlowfish = false
	}

	if doChecksum == true {
		// Add 4 empty bytes for the checksum new( new(
		data = append(data, []byte{0x00, 0x00, 0x00, 0x00}...)

		// Add blowfish padding
		missing := len(data) % 8

		if missing != 0 {
			for i := missing; i < 8; i++ {
				data = append(data, byte(0x00))
			}
		}

		// Finally do the checksum
		crypt.AppendChecksum(data)
	}

	if doBlowfish == true {
		var err error
		data, err = crypt.BlowfishEncrypt(data, c.BlowfishKey)

		if err != nil {
			return err
		}
	}

	// Calculate the packet length
	length := uint16(len(data) + 2)

	// Put everything together
	buffer := packets.NewBuffer()
	buffer.WriteUInt16(length)
	buffer.Write(data)

	_, err := c.Socket.Write(buffer.Bytes())

	if err != nil {
		return errors.New("The packet couldn't be sent.")
	}

	return nil
}

// SendStatic sends a packet encrypted with static Blowfish key and XOR pass (for Init packet)
func (c *Client) SendStatic(data []byte) error {
	// Add 4 empty bytes for the checksum
	data = append(data, []byte{0x00, 0x00, 0x00, 0x00}...)

	// Add padding for XOR (additional 4 bytes for XOR key)
	data = append(data, []byte{0x00, 0x00, 0x00, 0x00}...)

	// Add blowfish padding
	missing := len(data) % 8
	if missing != 0 {
		for i := missing; i < 8; i++ {
			data = append(data, byte(0x00))
		}
	}

	// Apply XOR pass encryption (like Java LoginCrypt does for static packets)
	// Generate random XOR key (as Java does with Rnd.nextInt())
	xorKey := mathrand.Uint32()
	crypt.EncXORPass(data, 0, len(data), xorKey)

	// Encrypt with static Blowfish key
	var err error
	data, err = crypt.BlowfishEncryptStatic(data)
	if err != nil {
		return err
	}

	// Calculate the packet length
	length := uint16(len(data) + 2)

	// Put everything together
	buffer := packets.NewBuffer()
	buffer.WriteUInt16(length)
	buffer.Write(data)

	_, err = c.Socket.Write(buffer.Bytes())
	if err != nil {
		return errors.New("The packet couldn't be sent.")
	}

	return nil
}

// RemoteAddr returns the remote address of the client connection
func (c *Client) RemoteAddr() string {
	if c.Socket != nil {
		return c.Socket.RemoteAddr().String()
	}
	return "unknown"
}
