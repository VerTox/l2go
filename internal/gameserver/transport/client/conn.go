package client

import (
    "encoding/binary"
    "errors"
    "fmt"
    "net"

    "github.com/VerTox/l2go/internal/gameserver/transport/gamecrypt"
)

// ClientConn wraps a game client TCP connection with optional XOR crypto.
type ClientConn struct {
    Conn   net.Conn
    crypt  *gamecrypt.Crypt
    enable bool
}

func NewClientConn(c net.Conn) *ClientConn { return &ClientConn{Conn: c, crypt: gamecrypt.New()} }

// EnableCrypt sets 16-byte L2J GameCrypt key (first 8 dynamic, last 8 static).
func (cc *ClientConn) EnableCrypt(key16 []byte) error {
    if len(key16) < 16 {
        return fmt.Errorf("gamecrypt key must be 16 bytes, got %d", len(key16))
    }
    cc.crypt.SetKey(key16)
    cc.enable = true
    return nil
}

// Receive reads a single packet: [len:2][payload...], returns opcode and payload (without opcode).
// If XOR is enabled, payload is decrypted before opcode extraction.
func (cc *ClientConn) Receive() (byte, []byte, error) {
    header := make([]byte, 2)
    if _, err := cc.readFull(header); err != nil {
        return 0, nil, fmt.Errorf("read header: %w", err)
    }
    size := int(binary.LittleEndian.Uint16(header))
    if size < 3 { // min length: 2 (len) + 1 (opcode)
        return 0, nil, errors.New("invalid packet size")
    }
    payload := make([]byte, size-2)
    if _, err := cc.readFull(payload); err != nil {
        return 0, nil, fmt.Errorf("read payload: %w", err)
    }
    if cc.enable {
        cc.crypt.Decrypt(payload)
    }
    opcode := payload[0]
    return opcode, payload[1:], nil
}

// Send writes a packet with length prefix and optional XOR of payload.
// data must include opcode as first byte.
func (cc *ClientConn) Send(data []byte) error {
    if len(data) == 0 {
        return errors.New("empty packet")
    }
    buf := make([]byte, 2+len(data))
    binary.LittleEndian.PutUint16(buf[:2], uint16(len(data)+2))
    copy(buf[2:], data)
    if cc.enable {
        // Encrypt payload in place (excluding length header)
        cc.crypt.Encrypt(buf[2:])
    }
    _, err := cc.Conn.Write(buf)
    return err
}

func (cc *ClientConn) readFull(b []byte) (int, error) {
    off := 0
    for off < len(b) {
        n, err := cc.Conn.Read(b[off:])
        if err != nil {
            return off, err
        }
        off += n
    }
    return off, nil
}

func (cc *ClientConn) Close() error { return cc.Conn.Close() }
