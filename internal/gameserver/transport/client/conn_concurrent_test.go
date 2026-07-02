package client

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/transport/gamecrypt"
)

// TestConcurrentSendKeepsStreamInSync reproduces the potion-spam corruption:
// the game loop and the client's handler goroutine both call Send on the same
// ClientConn. Send mutates the stateful XOR key, so without serialization the
// encrypt order and the wire order diverge and the receiver decodes garbage.
//
// Run with -race to also catch the data race on the cipher's outKey.
func TestConcurrentSendKeepsStreamInSync(t *testing.T) {
	const senders = 8
	const perSender = 40

	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i*7 + 1)
	}

	sc := NewClientConn(srv)
	if err := sc.EnableCrypt(key); err != nil {
		t.Fatalf("enable server crypt: %v", err)
	}

	// Receiver decrypts with an independent cipher seeded with the same key.
	// If the server's encrypt order matches the wire order, every frame decodes
	// to one of the payloads we sent; otherwise we get garbage.
	// Mirror the ClientConn cipher lifecycle: the first Encrypt call only flips
	// the cipher on (first packet goes in clear, key does not advance), so we
	// enable rxCrypt the same way and skip decrypting the first wire frame.
	rxCrypt := gamecrypt.New()
	rxCrypt.SetKey(key)
	rxCrypt.Encrypt(nil) // flip enabled without advancing the key

	total := senders * perSender
	payloads := make(map[string]bool, total)
	var pmu sync.Mutex

	// Reader goroutine: pull `total` framed packets off the pipe and decrypt.
	readErr := make(chan error, 1)
	got := make(chan []byte, total)
	go func() {
		header := make([]byte, 2)
		for i := 0; i < total; i++ {
			if _, err := readFullConn(cli, header); err != nil {
				readErr <- err
				return
			}
			size := int(binary.LittleEndian.Uint16(header))
			body := make([]byte, size-2)
			if _, err := readFullConn(cli, body); err != nil {
				readErr <- err
				return
			}
			// The very first frame on the wire is sent in clear (cipher enable
			// step), matching ClientConn.Send; decrypt every frame after it.
			if i > 0 {
				rxCrypt.Decrypt(body)
			}
			got <- body
		}
		readErr <- nil
	}()

	// Fan out concurrent senders, each with a distinct payload byte so we can
	// verify exact bytes survive the round trip.
	var wg sync.WaitGroup
	for s := 0; s < senders; s++ {
		wg.Add(1)
		go func(tag byte) {
			defer wg.Done()
			for n := 0; n < perSender; n++ {
				payload := []byte{tag, byte(n), 0xAB, 0xCD}
				pmu.Lock()
				payloads[string(payload)] = true
				pmu.Unlock()
				if err := sc.Send(payload); err != nil {
					t.Errorf("send: %v", err)
					return
				}
			}
		}(byte(s + 1))
	}
	wg.Wait()

	if err := <-readErr; err != nil {
		t.Fatalf("reader error: %v", err)
	}
	for i := 0; i < total; i++ {
		body := <-got
		pmu.Lock()
		ok := payloads[string(body)]
		pmu.Unlock()
		if !ok {
			t.Fatalf("decoded frame %x not among sent payloads — stream desynced", body)
		}
	}
	_ = bytes.Equal // keep import if payload comparison is refactored
}

func readFullConn(c net.Conn, b []byte) (int, error) {
	off := 0
	for off < len(b) {
		n, err := c.Read(b[off:])
		if err != nil {
			return off, err
		}
		off += n
	}
	return off, nil
}
