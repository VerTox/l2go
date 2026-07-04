package client

import (
    "encoding/binary"
    "errors"
    "fmt"
    "net"
    "sync"
    "time"

    "github.com/VerTox/l2go/internal/gameserver/transport/gamecrypt"
)

// sendQueueCapacity bounds a connection's outbound queue. Sized well above any
// legitimate per-tick burst (e.g. a mass spawn broadcasting many CharInfos to one
// client) so healthy clients never hit it; a client that lets this many packets
// back up is not reading — it is kicked rather than allowed to stall the loop.
const sendQueueCapacity = 1024

// writeTimeout bounds how long the writer goroutine may block on a single socket
// write, so a half-open/stuck client cannot wedge its writer indefinitely.
const writeTimeout = 30 * time.Second

// errSendQueueFull is returned by Send when the outbound queue is full and the
// connection is being closed as a slow/stuck client.
var errSendQueueFull = errors.New("send queue full: closing slow connection")

// ClientConn wraps a game client TCP connection with optional XOR crypto.
//
// Outbound packets are decoupled from the caller (l2go-e9q): Send enqueues ready
// bytes onto a bounded per-connection queue and returns immediately, and a single
// writer goroutine owns encryption (the stateful XOR outKey) and the socket write.
// This keeps the game-loop goroutine off the network path — a slow client can no
// longer stall a broadcast tick — while preserving strict per-connection packet
// order and sequential encryption (one queue, one writer).
type ClientConn struct {
    Conn   net.Conn
    crypt  *gamecrypt.Crypt
    enable bool

    sendCh    chan []byte
    done      chan struct{}
    closeOnce sync.Once
}

func NewClientConn(c net.Conn) *ClientConn {
    cc := &ClientConn{
        Conn:   c,
        crypt:  gamecrypt.New(),
        sendCh: make(chan []byte, sendQueueCapacity),
        done:   make(chan struct{}),
    }
    go cc.writeLoop()
    return cc
}

// EnableCrypt sets 16-byte L2J GameCrypt key (first 8 dynamic, last 8 static).
func (cc *ClientConn) EnableCrypt(key16 []byte) error {
    if len(key16) < 16 {
        return fmt.Errorf("gamecrypt key must be 16 bytes, got %d", len(key16))
    }
    cc.crypt.SetKey(key16)
    // Inbound decryption is enabled here, on the read-loop goroutine (the sole
    // Decrypt caller). Outbound encryption self-enables on the first Encrypt, which
    // runs on the writer goroutine — the two never share an enable flag. (l2go-e9q)
    cc.crypt.EnableDecrypt()
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

// Send enqueues a ready packet for asynchronous delivery. It does NOT block: if the
// queue is full the client is a slow/stuck reader and gets closed rather than being
// allowed to stall the caller (the game loop). data must include the opcode as the
// first byte and must not be mutated after the call — the writer reads it later.
// The same slice may be handed to several connections (broadcast reuse): the writer
// copies before its in-place XOR, so the shared bytes are never mutated. (l2go-e9q)
func (cc *ClientConn) Send(data []byte) error {
    if len(data) == 0 {
        return errors.New("empty packet")
    }
    select {
    case cc.sendCh <- data:
        return nil
    default:
        // Queue full → the client is not draining. Kick it; never block the caller.
        _ = cc.Close()
        return errSendQueueFull
    }
}

// writeLoop is the single per-connection writer. It owns the outbound cipher and
// the socket write, so encryption stays strictly sequential and packet order is
// preserved. It exits on a write error (closing the connection, which unblocks the
// read loop) or when the connection is closed.
func (cc *ClientConn) writeLoop() {
    for {
        select {
        case data := <-cc.sendCh:
            if err := cc.writePacket(data); err != nil {
                _ = cc.Close()
                return
            }
        case <-cc.done:
            cc.drain()
            return
        }
    }
}

// drain flushes packets already queued at close time (best-effort), so a final
// packet enqueued just before a graceful close (e.g. a leave-world notice) still
// goes out. Stops on the first write error or an empty queue.
func (cc *ClientConn) drain() {
    for {
        select {
        case data := <-cc.sendCh:
            if err := cc.writePacket(data); err != nil {
                return
            }
        default:
            return
        }
    }
}

// writePacket frames, encrypts (if enabled), and writes one packet. Runs only on
// the writer goroutine, so the stateful cipher needs no lock. It copies data into a
// fresh buffer before the in-place XOR, so a shared source slice is never mutated.
func (cc *ClientConn) writePacket(data []byte) error {
    buf := make([]byte, 2+len(data))
    binary.LittleEndian.PutUint16(buf[:2], uint16(len(data)+2))
    copy(buf[2:], data)
    if cc.enable {
        cc.crypt.Encrypt(buf[2:])
    }
    _ = cc.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))
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

// Close is idempotent: it signals the writer to stop and closes the socket. Safe to
// call from any goroutine (the loop's slow-client kick, the read loop's disconnect
// cleanup, and the login-replacement kick all may race).
func (cc *ClientConn) Close() error {
    cc.closeOnce.Do(func() {
        close(cc.done)
        _ = cc.Conn.Close()
    })
    return nil
}
