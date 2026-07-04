package gameloop

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/registry"
)

func TestResolveCombatTarget_None(t *testing.T) {
	gl := &GameLoop{world: registry.NewWorldRegistry()}
	if _, ok := gl.resolveCombatTarget(999999); ok {
		t.Fatal("expected no target for unknown objectID")
	}
}
