package stressclient

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/VerTox/l2go/internal/gameserver/transport/gamecrypt"
)

// gsConn is a framed connection to the GameServer. Framing mirrors
// internal/gameserver/transport/client/conn.go: 2-byte little-endian length
// prefix, payload XOR-encrypted with the shared 16-byte GameCrypt key once the
// handshake completes. Not safe for concurrent use.
type gsConn struct {
	conn   net.Conn
	crypt  *gamecrypt.Crypt
	active bool // once true, payloads are XOR-en/decrypted
}

func (c *gsConn) readFrame() ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	size := int(binary.LittleEndian.Uint16(header))
	if size < 3 {
		return nil, fmt.Errorf("bad frame size %d", size)
	}
	payload := make([]byte, size-2)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return nil, fmt.Errorf("read payload(%d): %w", size-2, err)
	}
	return payload, nil
}

// recv reads one packet, decrypting if the handshake is done. Returns opcode and
// the body after the opcode byte.
func (c *gsConn) recv() (byte, []byte, error) {
	payload, err := c.readFrame()
	if err != nil {
		return 0, nil, err
	}
	if c.active {
		c.crypt.Decrypt(payload)
	}
	if len(payload) == 0 {
		return 0, nil, fmt.Errorf("empty packet")
	}
	return payload[0], payload[1:], nil
}

// send writes one packet, encrypting the payload if the handshake is done. body
// must start with the opcode byte.
func (c *gsConn) send(body []byte) error {
	buf := make([]byte, 2+len(body))
	binary.LittleEndian.PutUint16(buf[:2], uint16(len(body)+2))
	copy(buf[2:], body)
	if c.active {
		c.crypt.Encrypt(buf[2:])
	}
	_, err := c.conn.Write(buf)
	return err
}

// recvUntil drains packets until one with the wanted opcode arrives (or the
// deadline fires). Handshake steps are strictly ordered on our server, but this
// tolerates an unexpected packet slipping in front of the one we await.
func (c *gsConn) recvUntil(want byte, maxDrain int) ([]byte, error) {
	for i := 0; i < maxDrain; i++ {
		op, body, err := c.recv()
		if err != nil {
			return nil, err
		}
		if op == want {
			return body, nil
		}
	}
	return nil, fmt.Errorf("packet 0x%02X not seen within %d packets", want, maxDrain)
}

// gameStaticKeyTail is the fixed second half of the 16-byte GameCrypt key; the
// KeyPacket only carries the first 8 bytes (see handler.go / L2J BlowFishKeygen).
var gameStaticKeyTail = []byte{0xc8, 0x27, 0x93, 0x01, 0xa1, 0x6c, 0x31, 0x97}
