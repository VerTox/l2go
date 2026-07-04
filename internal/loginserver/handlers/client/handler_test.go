package client

import (
	"testing"

	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/transport"
)

func newTestHandler() *Handler {
	return &Handler{
		clients:  make(map[*transport.Client]bool),
		accounts: make(map[string]*transport.Client),
	}
}

func TestRegisterAccount_ReturnsDisplacedClient(t *testing.T) {
	h := newTestHandler()
	c1 := &transport.Client{Account: &models.Account{Username: "bob"}}
	c2 := &transport.Client{Account: &models.Account{Username: "bob"}}

	if old := h.registerAccount("bob", c1); old != nil {
		t.Fatalf("first register should displace nothing, got %v", old)
	}
	if old := h.registerAccount("bob", c2); old != c1 {
		t.Fatalf("second register should displace c1, got %v", old)
	}
	if old := h.registerAccount("bob", c2); old != nil {
		t.Fatalf("re-register of same client should displace nothing, got %v", old)
	}
}

func TestRemoveClient_ConnAwareAccountCleanup(t *testing.T) {
	h := newTestHandler()
	c1 := &transport.Client{Account: &models.Account{Username: "bob"}}
	c2 := &transport.Client{Account: &models.Account{Username: "bob"}}
	h.accounts["bob"] = c1

	// c2 disconnecting must NOT remove c1's (newer) registration.
	h.removeClient(c2)
	if h.accounts["bob"] != c1 {
		t.Fatalf("stale client disconnect clobbered live registration")
	}

	// c1 disconnecting removes it.
	h.removeClient(c1)
	if _, ok := h.accounts["bob"]; ok {
		t.Fatalf("matching client disconnect should remove registration")
	}
}
