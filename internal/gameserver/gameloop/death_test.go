package gameloop

import (
	"testing"
)

// TestPlayerDeathClearsCombatStance verifies that a dying player leaves combat
// immediately. Otherwise InCombat stays set (until the 15s stance timeout) and both
// logout and restart are blocked — a dead player gets soft-locked. (l2go-3xh.1)
func TestPlayerDeathClearsCombatStance(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	_ = gl.world.SetPlayerCombatState(7, true)
	if !player.InCombat {
		t.Fatal("precondition: player should be in combat before death")
	}

	gl.handlePlayerDeath(7, player)

	if player.InCombat {
		t.Error("player must not be InCombat after death (otherwise logout/restart stay blocked)")
	}
}
