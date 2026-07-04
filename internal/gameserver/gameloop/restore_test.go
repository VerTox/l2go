package gameloop

import "testing"

// TestHandleRestoreStats_ClampsToMax verifies HP/MP/CP restore clamps at maxima.
func TestHandleRestoreStats_ClampsToMax(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	player.Character.MaxHP = 100
	player.Character.CurrentHP = 80
	player.Character.MaxMP = 50
	player.Character.CurrentMP = 10
	player.Character.MaxCP = 40
	player.Character.CurrentCP = 39

	// Restore more than the headroom for HP and CP; MP fits exactly.
	gl.handleRestoreStats(CmdRestoreStats{CharID: 7, HP: 50, MP: 40, CP: 100})

	if player.Character.CurrentHP != 100 {
		t.Errorf("CurrentHP = %v, want clamped to 100", player.Character.CurrentHP)
	}
	if player.Character.CurrentMP != 50 {
		t.Errorf("CurrentMP = %v, want 50", player.Character.CurrentMP)
	}
	if player.Character.CurrentCP != 40 {
		t.Errorf("CurrentCP = %v, want clamped to 40", player.Character.CurrentCP)
	}
}

// TestHandleRestoreStats_PartialRestore verifies a normal (non-clamping) restore.
func TestHandleRestoreStats_PartialRestore(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	player.Character.MaxHP = 500
	player.Character.CurrentHP = 100

	gl.handleRestoreStats(CmdRestoreStats{CharID: 7, HP: 50})

	if player.Character.CurrentHP != 150 {
		t.Errorf("CurrentHP = %v, want 150", player.Character.CurrentHP)
	}
}

// TestHandleRestoreStats_DeadPlayerIgnored verifies potions do not "resurrect".
func TestHandleRestoreStats_DeadPlayerIgnored(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	player.Character.MaxHP = 500
	player.Character.CurrentHP = 0 // dead

	gl.handleRestoreStats(CmdRestoreStats{CharID: 7, HP: 100})

	if player.Character.CurrentHP != 0 {
		t.Errorf("CurrentHP = %v, want 0 (dead player not healed by potion)", player.Character.CurrentHP)
	}
}

// TestItemSkillCaster_EnqueuesCommand verifies the adapter posts a CmdItemSkillCast.
func TestItemSkillCaster_EnqueuesCommand(t *testing.T) {
	gl, _ := newTestLoopWithPlayer(t)
	gl.ItemSkillCaster().CastItemSkill(7, 2037, 1)

	select {
	case cmd := <-gl.commands:
		c, ok := cmd.(CmdItemSkillCast)
		if !ok {
			t.Fatalf("got %T, want CmdItemSkillCast", cmd)
		}
		if c.CharID != 7 || c.SkillID != 2037 || c.Level != 1 {
			t.Errorf("cmd = %+v, want {7,2037,1}", c)
		}
	default:
		t.Fatal("no command enqueued")
	}
}
