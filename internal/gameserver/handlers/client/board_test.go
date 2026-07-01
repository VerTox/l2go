package client

import "testing"

// RequestShowMiniMap (0x6C) должен резолвиться в StateInGame реальным обработчиком.
func TestRequestShowMiniMapResolves(t *testing.T) {
	r := buildRegistry()
	e, ok := r.Resolve(StateInGame, 0x6c, 0)
	if !ok {
		t.Fatal("0x6c (RequestShowMiniMap) должен резолвиться в StateInGame")
	}
	if e.Name != "RequestShowMiniMap" {
		t.Errorf("Name = %q, want RequestShowMiniMap", e.Name)
	}
	if e.Handle == nil {
		t.Error("Handle не должен быть nil")
	}
}
