package registry

import (
	"sort"
	"testing"
)

func TestAutoShotRegistry_EnableListDisable(t *testing.T) {
	r := NewAutoShotRegistry()
	const char int32 = 7

	if r.IsActive(char, 1803) {
		t.Fatal("should start inactive")
	}

	r.Enable(char, 1803)
	r.Enable(char, 3947)
	if !r.IsActive(char, 1803) {
		t.Error("1803 should be active after Enable")
	}

	got := r.List(char)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if len(got) != 2 || got[0] != 1803 || got[1] != 3947 {
		t.Errorf("List = %v, want [1803 3947]", got)
	}

	r.Disable(char, 1803)
	if r.IsActive(char, 1803) {
		t.Error("1803 should be inactive after Disable")
	}
	if len(r.List(char)) != 1 {
		t.Errorf("List after disable = %v, want 1 item", r.List(char))
	}
}

// The active list is per-character and reset on relog (L2J does not persist it).
func TestAutoShotRegistry_ClearOnRelog(t *testing.T) {
	r := NewAutoShotRegistry()
	r.Enable(7, 1803)
	r.Enable(7, 3947)
	r.Enable(8, 1803) // another char unaffected by clearing char 7

	r.Clear(7)

	if len(r.List(7)) != 0 {
		t.Errorf("List(7) after Clear = %v, want empty", r.List(7))
	}
	if r.IsActive(7, 1803) {
		t.Error("char 7 should be inactive after Clear")
	}
	if !r.IsActive(8, 1803) {
		t.Error("char 8 must be unaffected by clearing char 7")
	}
}
