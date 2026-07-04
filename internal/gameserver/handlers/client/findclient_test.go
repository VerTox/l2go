package client

import (
	"strconv"
	"sync"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

// TestFindClientByAccount_Concurrent proves the account lookup is race-free under
// concurrent connect/disconnect churn — the previous h.sessions iteration crashed
// the process with "concurrent map iteration and map write". Run with -race. (l2go-7c0)
func TestFindClientByAccount_Concurrent(t *testing.T) {
	h := &Handler{connections: registry.NewConnectionRegistry()}
	c1 := &client.ClientConn{}
	h.connections.Register("bob", c1)

	if got := h.findClientByAccount("bob"); got != c1 {
		t.Fatalf("findClientByAccount(bob) = %v, want c1", got)
	}

	var wg sync.WaitGroup
	// Readers: hammer the lookup.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2000; j++ {
				_ = h.findClientByAccount("bob")
			}
		}()
	}
	// Writers: churn other accounts (connect/disconnect).
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			acc := "w" + strconv.Itoa(id)
			conn := &client.ClientConn{}
			for j := 0; j < 2000; j++ {
				h.connections.Register(acc, conn)
				h.connections.UnregisterIf(acc, conn)
			}
		}(i)
	}
	wg.Wait()

	if got := h.findClientByAccount("bob"); got != c1 {
		t.Fatalf("after churn findClientByAccount(bob) = %v, want c1", got)
	}
}
