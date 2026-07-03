package gameloop

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// TestRegenPlayers_RestoresAndClamps verifies a living player regenerates HP/MP/CP
// by the per-level amount, clamped to the maxima.
func TestRegenPlayers_RestoresAndClamps(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	c := player.Character
	c.Level = 1
	c.MaxHP, c.CurrentHP = 500, 100
	c.MaxMP, c.CurrentMP = 200, 50
	c.MaxCP, c.CurrentCP = 300, 100

	gl.regenPlayers()

	// Level 1 regen: HP +2.0, MP +0.9, CP +2 (int).
	if c.CurrentHP != 100+models.HpRegenPerTick(1) {
		t.Errorf("CurrentHP = %v, want %v", c.CurrentHP, 100+models.HpRegenPerTick(1))
	}
	if c.CurrentMP != 50+models.MpRegenPerTick(1) {
		t.Errorf("CurrentMP = %v, want %v", c.CurrentMP, 50+models.MpRegenPerTick(1))
	}
	if c.CurrentCP != 100+int(models.CpRegenPerTick(1)) {
		t.Errorf("CurrentCP = %v, want %v", c.CurrentCP, 100+int(models.CpRegenPerTick(1)))
	}

	// Near-full: regen clamps to max, never overshoots.
	c.CurrentHP = float64(c.MaxHP) - 0.5
	c.CurrentMP = float64(c.MaxMP)
	c.CurrentCP = c.MaxCP
	gl.regenPlayers()
	if c.CurrentHP != float64(c.MaxHP) {
		t.Errorf("CurrentHP = %v, want clamped to %d", c.CurrentHP, c.MaxHP)
	}
	if c.CurrentMP != float64(c.MaxMP) || c.CurrentCP != c.MaxCP {
		t.Errorf("overshoot: MP=%v CP=%d", c.CurrentMP, c.CurrentCP)
	}
}

// TestRegenPlayers_DeadSkipped verifies a dead player does not regenerate.
func TestRegenPlayers_DeadSkipped(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	c := player.Character
	c.MaxHP, c.CurrentHP = 500, 0 // dead
	c.MaxMP, c.CurrentMP = 200, 10

	gl.regenPlayers()

	if c.CurrentHP != 0 {
		t.Errorf("dead player CurrentHP = %v, want 0 (no regen)", c.CurrentHP)
	}
	if c.CurrentMP != 10 {
		t.Errorf("dead player CurrentMP = %v, want 10 (no regen)", c.CurrentMP)
	}
}

// TestRegenPlayers_FullNoChange verifies a fully-topped player is left untouched.
func TestRegenPlayers_FullNoChange(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	c := player.Character
	c.MaxHP, c.CurrentHP = 100, 100
	c.MaxMP, c.CurrentMP = 100, 100
	c.MaxCP, c.CurrentCP = 100, 100

	gl.regenPlayers()

	if c.CurrentHP != 100 || c.CurrentMP != 100 || c.CurrentCP != 100 {
		t.Errorf("full player changed: HP=%v MP=%v CP=%d", c.CurrentHP, c.CurrentMP, c.CurrentCP)
	}
}
