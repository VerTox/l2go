package stressclient

import (
	"fmt"
	"net"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/transport/gamecrypt"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// GS -> client opcodes used in the handshake.
const (
	gsKeyPacket         = 0x2e
	gsCharSelectionInfo = 0x09
	gsCharSelected      = 0x0b
)

// protocolVersion is the value we announce; our server parses but ignores it.
const protocolVersion = 268

// GameSession is a live, authenticated in-world connection. Sends (Walk/Say) are
// NOT safe for concurrent use — drive them from a single behaviour goroutine;
// Drain (reads) may run in a separate goroutine (net.Conn allows concurrent
// read+write, and the XOR in/out keys are independent).
type GameSession struct {
	c *gsConn
	// spawn position parsed from CharSelected; the behaviour goroutine tracks
	// its own position from here (a faithful client, unlike the headless node lib).
	spawnX, spawnY, spawnZ int32
	posX, posY, posZ       int32
}

// Spawn returns the character's spawn position (from CharSelected).
func (s *GameSession) Spawn() (x, y, z int32) { return s.spawnX, s.spawnY, s.spawnZ }

// EnterWorld runs the full GameServer handshake for a character slot and returns
// a live in-world session. keys come from Login. gsAddr is the GameServer address
// to dial (the ServerList advertises the server's own IP, which may be an internal
// docker address, so the caller passes the reachable one).
func EnterWorld(gsAddr, account string, slot int, keys *SessionKeys, timeout time.Duration) (*GameSession, error) {
	conn, err := net.DialTimeout("tcp", gsAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial gs: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	c := &gsConn{conn: conn, crypt: gamecrypt.New()}

	// 1. ProtocolVersion (unencrypted).
	pv := l2pkt.NewWriter()
	pv.WriteC(0x0e)
	pv.WriteD(protocolVersion)
	if err := c.send(pv.Bytes()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send ProtocolVersion: %w", err)
	}

	// 2. KeyPacket (unencrypted): carries the first 8 bytes of the XOR key.
	op, body, err := c.recv()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("recv KeyPacket: %w", err)
	}
	if op != gsKeyPacket || len(body) < 9 {
		conn.Close()
		return nil, fmt.Errorf("expected KeyPacket, got 0x%02X len=%d", op, len(body))
	}
	key := make([]byte, 0, 16)
	key = append(key, body[1:9]...) // body[0] is the id byte, key follows
	key = append(key, gameStaticKeyTail...)
	c.crypt.SetKey(key)
	c.crypt.Encrypt(nil) // prime: flips GameCrypt enabled without advancing the key
	c.active = true

	// 3. AuthLogin: account + play/login session keys (order per server's Read:
	// account, playKey2, playKey1, loginKey1, loginKey2).
	al := l2pkt.NewWriter()
	al.WriteC(0x2b)
	al.WriteS(account)
	al.WriteD(int32(keys.PlayOK2))
	al.WriteD(int32(keys.PlayOK1))
	al.WriteD(int32(keys.LoginOK1))
	al.WriteD(int32(keys.LoginOK2))
	if err := c.send(al.Bytes()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send AuthLogin: %w", err)
	}

	// 4. CharSelectionInfo (server validates the keys with the LoginServer first).
	if _, err := c.recvUntil(gsCharSelectionInfo, 8); err != nil {
		conn.Close()
		return nil, fmt.Errorf("recv CharSelectionInfo: %w", err)
	}

	// 5. CharacterSelect(slot). Body: slot(D) + H + D + D + D (server ignores the trailer).
	cs := l2pkt.NewWriter()
	cs.WriteC(0x12)
	cs.WriteD(int32(slot))
	cs.WriteH(0)
	cs.WriteD(0)
	cs.WriteD(0)
	cs.WriteD(0)
	if err := c.send(cs.Bytes()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send CharacterSelect: %w", err)
	}

	// 6. CharSelected — carries the spawn position we track locally.
	csBody, err := c.recvUntil(gsCharSelected, 8)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("recv CharSelected: %w", err)
	}
	sx, sy, sz := parseCharSelectedPos(csBody)

	// 7. EnterWorld (server ignores the body).
	ew := l2pkt.NewWriter()
	ew.WriteC(0x11)
	if err := c.send(ew.Bytes()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send EnterWorld: %w", err)
	}

	// 8. Confirm we are in-world by receiving one server packet (UserInfo/ItemList/
	// SkillList/... — we don't care which, just that the flood started).
	if _, _, err := c.recv(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("recv post-EnterWorld: %w", err)
	}

	// Clear the handshake deadline; the session lives until Close.
	_ = conn.SetDeadline(time.Time{})
	return &GameSession{c: c, spawnX: sx, spawnY: sy, spawnZ: sz, posX: sx, posY: sy, posZ: sz}, nil
}

// parseCharSelectedPos extracts X/Y/Z from a CharSelected body (after opcode).
// Layout: S(name) D(objId) S(title) D(sessionId) D(clanId) D(0) D(sex) D(race)
// D(classId) D(active) D(x) D(y) D(z) ... Reuses l2pkt.Reader for the UTF-16
// strings. Returns zeros if the packet is malformed (bot still works, just walks
// around origin).
func parseCharSelectedPos(body []byte) (x, y, z int32) {
	r := l2pkt.NewReader(body)
	_, _ = r.ReadS() // name
	_, _ = r.ReadD() // objectId
	_, _ = r.ReadS() // title
	for i := 0; i < 7; i++ {
		_, _ = r.ReadD() // sessionId, clanId, 0, sex, race, classId, active
	}
	x, _ = r.ReadD()
	y, _ = r.ReadD()
	z, _ = r.ReadD()
	return x, y, z
}

// Walk sends a MoveBackwardToLocation from the tracked position to (x,y,z) and
// advances the tracked position to the destination — behaving like a faithful
// client that reaches its target before the next move (WALK interval >= travel).
// Must be called from the single behaviour goroutine.
func (s *GameSession) Walk(x, y, z int32) error {
	w := l2pkt.NewWriter()
	w.WriteC(0x0f)
	w.WriteD(x)       // target
	w.WriteD(y)
	w.WriteD(z)
	w.WriteD(s.posX)  // origin = where we are now
	w.WriteD(s.posY)
	w.WriteD(s.posZ)
	w.WriteD(1) // moveMovement (1 = keyboard/ground move)
	if err := s.c.send(w.Bytes()); err != nil {
		return err
	}
	s.posX, s.posY, s.posZ = x, y, z
	return nil
}

// Say sends a Say2 general-chat (ALL channel) message. Must be called from the
// single behaviour goroutine.
func (s *GameSession) Say(text string) error {
	w := l2pkt.NewWriter()
	w.WriteC(0x49)
	w.WriteS(text)
	w.WriteD(0) // chat type 0 = ALL (local)
	return s.c.send(w.Bytes())
}

// Close ends the session.
func (s *GameSession) Close() error { return s.c.conn.Close() }

// Drain reads and discards one incoming packet, keeping the XOR stream in sync
// and the connection alive. Bots that only need to stay online call this in a
// loop. A read deadline should be set by the caller if it wants a timeout.
func (s *GameSession) Drain() error {
	_, _, err := s.c.recv()
	return err
}
