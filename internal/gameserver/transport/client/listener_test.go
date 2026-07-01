package client

import (
	"context"
	"testing"
	"time"
)

// TestListenAndServeStopsOnContextCancel verifies the listener returns promptly when
// the context is cancelled, instead of blocking forever in Accept(). A blocked Accept
// keeps the errgroup goroutine alive and makes graceful shutdown time out. (l2go-rs3)
func TestListenAndServeStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- ListenAndServe(ctx, "127.0.0.1:0", func(context.Context, *ClientConn) {})
	}()

	// Give the listener a moment to reach Accept().
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected nil on graceful stop, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ListenAndServe did not return after context cancel (blocked in Accept)")
	}
}
