package client

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

// TestClientConn_PreservesSendOrder verifies the async writer delivers packets in
// enqueue order (single FIFO queue, single writer). No crypt → plaintext frames.
func TestClientConn_PreservesSendOrder(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	sc := NewClientConn(srv)
	defer sc.Close()

	const n = 50
	got := make(chan byte, n)
	go func() {
		header := make([]byte, 2)
		for i := 0; i < n; i++ {
			if _, err := readFullConn(cli, header); err != nil {
				return
			}
			size := int(binary.LittleEndian.Uint16(header))
			body := make([]byte, size-2)
			if _, err := readFullConn(cli, body); err != nil {
				return
			}
			got <- body[0]
		}
	}()

	for i := 0; i < n; i++ {
		if err := sc.Send([]byte{byte(i)}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	for i := 0; i < n; i++ {
		select {
		case b := <-got:
			if b != byte(i) {
				t.Fatalf("out of order: frame %d = %d, want %d", i, b, i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for frame %d", i)
		}
	}
}

// TestClientConn_KicksSlowClientWithoutBlocking is the core l2go-e9q guarantee: a
// client that never reads must not stall the caller. Send stays non-blocking and,
// once the bounded queue fills, closes the connection instead of blocking.
func TestClientConn_KicksSlowClientWithoutBlocking(t *testing.T) {
	srv, cli := net.Pipe()
	defer cli.Close()

	sc := NewClientConn(srv)

	// cli is never read, so the writer blocks on its first Write and the queue
	// fills. Bounded loop + timeout guard: if Send ever blocked, this would hang.
	done := make(chan bool, 1)
	go func() {
		for i := 0; i < sendQueueCapacity+100; i++ {
			err := sc.Send([]byte{1, 2, 3})
			if err != nil {
				done <- errors.Is(err, errSendQueueFull)
				return
			}
		}
		done <- false // never kicked
	}()

	select {
	case kicked := <-done:
		if !kicked {
			t.Fatal("queue overflow did not kick the slow client")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Send blocked on a slow client — the loop would stall")
	}

	// The connection must be closed after the overflow kick.
	select {
	case <-sc.done:
	case <-time.After(time.Second):
		t.Error("connection not closed after overflow")
	}
}

// TestClientConn_SendAfterCloseIsSafe checks that sending after Close never panics
// (the send channel is never closed) and never blocks.
func TestClientConn_SendAfterCloseIsSafe(t *testing.T) {
	srv, cli := net.Pipe()
	defer cli.Close()

	sc := NewClientConn(srv)
	_ = sc.Close()
	_ = sc.Close() // idempotent

	done := make(chan struct{})
	go func() {
		for i := 0; i < sendQueueCapacity+10; i++ {
			_ = sc.Send([]byte{1}) // may buffer or report full; must not panic/block
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Send blocked after Close")
	}
}
