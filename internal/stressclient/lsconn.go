// Package stressclient is a native Go Lineage II client used only for load
// testing: one goroutine per bot instead of the Node harness's one-process-per-bot
// (~40MB each), so 1000+ bots fit in a few MB. It reuses the server's own crypto
// (pkg/crypt) and mirrors the protocol byte layouts defined under
// internal/*/packets, guaranteeing byte-compatibility with our server.
//
// This file implements the LoginServer wire framing: the Init packet (static
// Blowfish + XOR pass) and all subsequent packets (dynamic Blowfish + checksum),
// mirroring internal/loginserver/transport/client.go.
package stressclient

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/VerTox/l2go/pkg/crypt"
)

// lsConn is a framed connection to the LoginServer. Not safe for concurrent use;
// each bot owns its own.
type lsConn struct {
	conn     net.Conn
	blowfish []byte // dynamic Blowfish key learned from Init
}

// readFrame reads one length-prefixed frame and returns the still-encrypted body.
func (c *lsConn) readFrame() ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	size := int(header[0]) | int(header[1])<<8
	if size < 2 {
		return nil, fmt.Errorf("bad frame size %d", size)
	}
	payload := make([]byte, size-2)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return nil, fmt.Errorf("read payload(%d): %w", size-2, err)
	}
	return payload, nil
}

// recvInit reads the first server packet: static-Blowfish encrypted with a
// reversible XOR pass over the body (SendStatic on the server side).
func (c *lsConn) recvInit() ([]byte, error) {
	payload, err := c.readFrame()
	if err != nil {
		return nil, err
	}
	dec, err := crypt.BlowfishDecrypt(payload, crypt.StaticBlowfishKey)
	if err != nil {
		return nil, fmt.Errorf("init blowfish: %w", err)
	}
	crypt.DecXORPass(dec, 0, len(dec))
	return dec, nil
}

// recv reads a normal packet (dynamic Blowfish). Returns opcode and the body
// after the opcode byte.
func (c *lsConn) recv() (byte, []byte, error) {
	payload, err := c.readFrame()
	if err != nil {
		return 0, nil, err
	}
	dec, err := crypt.BlowfishDecrypt(payload, c.blowfish)
	if err != nil {
		return 0, nil, fmt.Errorf("blowfish: %w", err)
	}
	if len(dec) == 0 {
		return 0, nil, fmt.Errorf("empty packet")
	}
	return dec[0], dec[1:], nil
}

// send writes a client packet: append checksum space, pad to 8, checksum, then
// dynamic Blowfish — mirroring the server's Send so the server decodes it. body
// must start with the opcode byte.
func (c *lsConn) send(body []byte) error {
	data := make([]byte, len(body))
	copy(data, body)
	data = append(data, 0, 0, 0, 0) // checksum space
	if missing := len(data) % 8; missing != 0 {
		for i := missing; i < 8; i++ {
			data = append(data, 0)
		}
	}
	crypt.AppendChecksum(data)
	enc, err := crypt.BlowfishEncrypt(data, c.blowfish)
	if err != nil {
		return fmt.Errorf("blowfish encrypt: %w", err)
	}
	frame := make([]byte, 2+len(enc))
	binary.LittleEndian.PutUint16(frame, uint16(len(enc)+2))
	copy(frame[2:], enc)
	_, err = c.conn.Write(frame)
	return err
}
