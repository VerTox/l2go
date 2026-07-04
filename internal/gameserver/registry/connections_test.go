package registry

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

func TestUnregisterIf_RemovesOnlyMatchingConn(t *testing.T) {
	cr := NewConnectionRegistry()
	c1 := &client.ClientConn{}
	c2 := &client.ClientConn{}

	cr.Register("bob", c1)

	// Stale conn c2 must NOT remove c1's registration.
	cr.UnregisterIf("bob", c2)
	if got := cr.GetConnection("bob"); got != c1 {
		t.Fatalf("expected c1 to survive UnregisterIf with stale conn, got %v", got)
	}

	// Matching conn removes it.
	cr.UnregisterIf("bob", c1)
	if got := cr.GetConnection("bob"); got != nil {
		t.Fatalf("expected nil after UnregisterIf with matching conn, got %v", got)
	}
}
