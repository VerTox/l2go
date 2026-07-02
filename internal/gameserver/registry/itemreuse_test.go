package registry

import (
	"testing"
	"time"
)

func TestItemReuse_AddAndRemainingByItem(t *testing.T) {
	r := NewItemReuseRegistry()
	base := time.Unix(1_000_000, 0)

	// No stamp yet -> 0.
	if got := r.RemainingByItem(7, 500, base); got != 0 {
		t.Fatalf("remaining before add = %v, want 0", got)
	}

	r.Add(7, 500, 0, 10*time.Second, base)

	if got := r.RemainingByItem(7, 500, base); got != 10*time.Second {
		t.Errorf("remaining at t0 = %v, want 10s", got)
	}
	if got := r.RemainingByItem(7, 500, base.Add(4*time.Second)); got != 6*time.Second {
		t.Errorf("remaining at t+4 = %v, want 6s", got)
	}
	// Expired -> 0, not negative.
	if got := r.RemainingByItem(7, 500, base.Add(10*time.Second)); got != 0 {
		t.Errorf("remaining at expiry = %v, want 0", got)
	}
	if got := r.RemainingByItem(7, 500, base.Add(30*time.Second)); got != 0 {
		t.Errorf("remaining after expiry = %v, want 0", got)
	}
}

func TestItemReuse_PerCharacterIsolation(t *testing.T) {
	r := NewItemReuseRegistry()
	base := time.Unix(2_000_000, 0)
	r.Add(1, 500, 0, 10*time.Second, base)

	if got := r.RemainingByItem(2, 500, base); got != 0 {
		t.Errorf("other char sees cooldown = %v, want 0", got)
	}
}

func TestItemReuse_SharedGroup(t *testing.T) {
	r := NewItemReuseRegistry()
	base := time.Unix(3_000_000, 0)

	// Two different item instances in the same shared reuse group (77).
	r.Add(9, 501, 77, 8*time.Second, base)

	// A sibling item (different objectID) shares the cooldown via the group.
	if got := r.RemainingByGroup(9, 77, base); got != 8*time.Second {
		t.Errorf("group remaining = %v, want 8s", got)
	}
	if got := r.RemainingByGroup(9, 77, base.Add(3*time.Second)); got != 5*time.Second {
		t.Errorf("group remaining at t+3 = %v, want 5s", got)
	}
	// Different group -> 0.
	if got := r.RemainingByGroup(9, 78, base); got != 0 {
		t.Errorf("other group remaining = %v, want 0", got)
	}
	// group<=0 never matches.
	if got := r.RemainingByGroup(9, 0, base); got != 0 {
		t.Errorf("group 0 remaining = %v, want 0", got)
	}
	// Expired group entry -> 0.
	if got := r.RemainingByGroup(9, 77, base.Add(8*time.Second)); got != 0 {
		t.Errorf("group remaining at expiry = %v, want 0", got)
	}
}

func TestItemReuse_GroupReturnsLargestRemaining(t *testing.T) {
	r := NewItemReuseRegistry()
	base := time.Unix(4_000_000, 0)
	r.Add(3, 600, 5, 4*time.Second, base)
	r.Add(3, 601, 5, 9*time.Second, base)

	if got := r.RemainingByGroup(3, 5, base); got != 9*time.Second {
		t.Errorf("group remaining = %v, want 9s (largest)", got)
	}
}

func TestItemReuse_Clear(t *testing.T) {
	r := NewItemReuseRegistry()
	base := time.Unix(5_000_000, 0)
	r.Add(4, 700, 12, 10*time.Second, base)
	r.Clear(4)

	if got := r.RemainingByItem(4, 700, base); got != 0 {
		t.Errorf("remaining after clear = %v, want 0", got)
	}
	if got := r.RemainingByGroup(4, 12, base); got != 0 {
		t.Errorf("group remaining after clear = %v, want 0", got)
	}
}

func TestItemReuse_NonPositiveTotalIgnored(t *testing.T) {
	r := NewItemReuseRegistry()
	base := time.Unix(6_000_000, 0)
	r.Add(5, 800, 0, 0, base)
	if got := r.RemainingByItem(5, 800, base); got != 0 {
		t.Errorf("remaining for zero-total = %v, want 0", got)
	}
}
