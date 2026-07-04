package client

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/transport/gamecrypt"
)

// TestAppendFramed_ByteIdenticalToSequential proves the core batching invariant
// (l2go-20j): coalescing N packets into one buffer produces the exact same bytes as
// framing+encrypting each packet on its own, in order. Each packet is encrypted
// separately (outKey advances per packet); only the socket write is batched, so the
// wire stream is unchanged and the client decrypts packet-by-packet as before.
func TestAppendFramed_ByteIdenticalToSequential(t *testing.T) {
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i*3 + 5)
	}

	// Subject: a connection whose appendFramed we exercise directly (no writer
	// goroutine — we build the struct rather than NewClientConn).
	cc := &ClientConn{crypt: gamecrypt.New()}
	if err := cc.EnableCrypt(key); err != nil {
		t.Fatalf("EnableCrypt: %v", err)
	}

	packets := [][]byte{
		{0x01, 0xAA, 0xBB},
		{0x2f, 1, 2, 3, 4, 5, 6, 7, 8},
		{0x18, 0xDE, 0xAD},
		{0x54},
	}

	// Batch: append all into one buffer.
	var batch []byte
	for _, p := range packets {
		batch = cc.appendFramed(batch, p)
	}

	// Reference: an independent cipher, framing + encrypting each packet in order.
	ref := gamecrypt.New()
	ref.SetKey(key)
	var seq []byte
	for _, p := range packets {
		frame := make([]byte, 2+len(p))
		binary.LittleEndian.PutUint16(frame[:2], uint16(len(p)+2))
		copy(frame[2:], p)
		ref.Encrypt(frame[2:]) // first call flips enable (clear), rest encrypt — matches appendFramed
		seq = append(seq, frame...)
	}

	if !bytes.Equal(batch, seq) {
		t.Fatalf("batched bytes differ from sequential framing\n batch=%x\n seq  =%x", batch, seq)
	}
}

// TestAppendFramed_DecryptsInOrder round-trips the batched buffer through a mirror
// cipher (as the real client would) and checks every packet comes back intact and in
// order — the first packet in clear, the rest decrypted.
func TestAppendFramed_DecryptsInOrder(t *testing.T) {
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i*11 + 2)
	}
	cc := &ClientConn{crypt: gamecrypt.New()}
	if err := cc.EnableCrypt(key); err != nil {
		t.Fatal(err)
	}

	payloads := [][]byte{
		{0x0a, 1, 1},
		{0x0b, 2, 2, 2},
		{0x0c, 3},
	}
	var batch []byte
	for _, p := range payloads {
		batch = cc.appendFramed(batch, p)
	}

	rx := gamecrypt.New()
	rx.SetKey(key)
	rx.EnableDecrypt()

	off := 0
	for i, want := range payloads {
		size := int(binary.LittleEndian.Uint16(batch[off : off+2]))
		body := append([]byte(nil), batch[off+2:off+size]...)
		if i > 0 { // first packet is sent in clear
			rx.Decrypt(body)
		}
		if !bytes.Equal(body, want) {
			t.Fatalf("packet %d decoded %x, want %x", i, body, want)
		}
		off += size
	}
	if off != len(batch) {
		t.Errorf("trailing bytes: consumed %d of %d", off, len(batch))
	}
}
