package stressclient

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
	"time"
)

// LS -> client opcodes.
const (
	opInit       = 0x00
	opLoginFail  = 0x01
	opLoginOk    = 0x03
	opServerList = 0x04
	opPlayFail   = 0x06
	opPlayOk     = 0x07
	opGGAuth     = 0x0B
)

// SessionKeys holds the four 32-bit session keys the GameServer auth needs, plus
// the chosen game server address.
type SessionKeys struct {
	LoginOK1, LoginOK2 uint32
	PlayOK1, PlayOK2   uint32
	loginOKRaw         []byte // first 8 bytes of LoginOk, sent verbatim to RequestServerList/Login
	ServerID           int
	GameIP             string
	GamePort           int
}

// Login runs the full LoginServer flow for one account and returns the session
// keys needed to authenticate to the GameServer. It stops at PlayOk (the LS side).
func Login(addr, username, password string, timeout time.Duration) (*SessionKeys, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	c := &lsConn{conn: conn}

	// 1. Init: RSA modulus + dynamic Blowfish key.
	init, err := c.recvInit()
	if err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	if len(init) < 169 || init[0] != opInit {
		return nil, fmt.Errorf("unexpected init packet (len=%d op=0x%02X)", len(init), safeOp(init))
	}
	sessionID := binary.LittleEndian.Uint32(init[1:5])
	scrambledMod := make([]byte, 128)
	copy(scrambledMod, init[9:137])
	modulus := unscrambleModulus(scrambledMod)
	c.blowfish = make([]byte, 16)
	copy(c.blowfish, init[153:169])

	// 2. AuthGameGuard: echo the session id, zeros for the rest.
	agg := make([]byte, 1+4+16)
	agg[0] = 0x07
	binary.LittleEndian.PutUint32(agg[1:], sessionID)
	if err := c.send(agg); err != nil {
		return nil, fmt.Errorf("send AuthGameGuard: %w", err)
	}
	if op, _, err := c.recv(); err != nil {
		return nil, fmt.Errorf("recv GGAuth: %w", err)
	} else if op != opGGAuth {
		return nil, fmt.Errorf("expected GGAuth, got 0x%02X", op)
	}

	// 3. RequestAuthLogin: RSA-encrypted credential block (user@0x5E, pass@0x6C).
	block := make([]byte, 128)
	copy(block[0x5E:], []byte(username)) // server reads [0x5E:0x5E+14]
	copy(block[0x6C:], []byte(password)) // server reads [0x6C:0x6C+16]
	cipher := rsaEncryptNoPad(modulus, block)
	ral := make([]byte, 1+128)
	ral[0] = 0x00
	copy(ral[1:], cipher)
	if err := c.send(ral); err != nil {
		return nil, fmt.Errorf("send RequestAuthLogin: %w", err)
	}
	op, body, err := c.recv()
	if err != nil {
		return nil, fmt.Errorf("recv LoginOk: %w", err)
	}
	if op == opLoginFail {
		return nil, fmt.Errorf("login failed (reason 0x%02X)", firstByte(body))
	}
	if op != opLoginOk || len(body) < 8 {
		return nil, fmt.Errorf("expected LoginOk, got 0x%02X len=%d", op, len(body))
	}
	keys := &SessionKeys{
		LoginOK1:   binary.LittleEndian.Uint32(body[0:4]),
		LoginOK2:   binary.LittleEndian.Uint32(body[4:8]),
		loginOKRaw: append([]byte(nil), body[0:8]...),
	}

	// 4. RequestServerList.
	rsl := append([]byte{0x05}, keys.loginOKRaw...)
	if err := c.send(rsl); err != nil {
		return nil, fmt.Errorf("send RequestServerList: %w", err)
	}
	op, body, err = c.recv()
	if err != nil {
		return nil, fmt.Errorf("recv ServerList: %w", err)
	}
	if op != opServerList || len(body) < 2 {
		return nil, fmt.Errorf("expected ServerList, got 0x%02X", op)
	}
	count := int(body[0])
	if count < 1 || len(body) < 2+11 {
		return nil, fmt.Errorf("empty/short ServerList (count=%d)", count)
	}
	// First server record: id(1) ip(4) port(4) ...
	rec := body[2:]
	keys.ServerID = int(rec[0])
	keys.GameIP = fmt.Sprintf("%d.%d.%d.%d", rec[1], rec[2], rec[3], rec[4])
	keys.GamePort = int(binary.LittleEndian.Uint32(rec[5:9]))

	// 5. RequestServerLogin.
	rsli := append(append([]byte{0x02}, keys.loginOKRaw...), byte(keys.ServerID))
	if err := c.send(rsli); err != nil {
		return nil, fmt.Errorf("send RequestServerLogin: %w", err)
	}
	op, body, err = c.recv()
	if err != nil {
		return nil, fmt.Errorf("recv PlayOk: %w", err)
	}
	if op == opPlayFail {
		return nil, fmt.Errorf("play failed (reason 0x%02X)", firstByte(body))
	}
	if op != opPlayOk || len(body) < 8 {
		return nil, fmt.Errorf("expected PlayOk, got 0x%02X len=%d", op, len(body))
	}
	keys.PlayOK1 = binary.LittleEndian.Uint32(body[0:4])
	keys.PlayOK2 = binary.LittleEndian.Uint32(body[4:8])

	return keys, nil
}

// rsaEncryptNoPad computes m^e mod n (e=65537) and left-pads to 128 bytes — the
// inverse of the server's RsaDecryptNoPadding.
func rsaEncryptNoPad(modulus, block []byte) []byte {
	n := new(big.Int).SetBytes(modulus)
	e := big.NewInt(65537)
	m := new(big.Int).SetBytes(block)
	c := new(big.Int).Exp(m, e, n)
	out := c.Bytes()
	if len(out) < 128 {
		out = append(make([]byte, 128-len(out)), out...)
	}
	return out
}

// unscrambleModulus reverses the server's ScrambleModulus (see
// internal/loginserver/packets: UnscrambleModulus). Operates in place on a
// 128-byte slice and returns it.
func unscrambleModulus(mods []byte) []byte {
	if len(mods) != 128 {
		return mods
	}
	for i := 0; i < 0x40; i++ {
		mods[0x40+i] ^= mods[i]
	}
	for i := 0; i < 4; i++ {
		mods[0x0D+i] ^= mods[0x34+i]
	}
	for i := 0; i < 0x40; i++ {
		mods[i] ^= mods[0x40+i]
	}
	for i := 0; i < 4; i++ {
		mods[0x00+i], mods[0x4D+i] = mods[0x4D+i], mods[0x00+i]
	}
	return mods
}

func safeOp(b []byte) byte {
	if len(b) == 0 {
		return 0xFF
	}
	return b[0]
}

func firstByte(b []byte) byte {
	if len(b) == 0 {
		return 0xFF
	}
	return b[0]
}
