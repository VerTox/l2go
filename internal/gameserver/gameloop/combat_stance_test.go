package gameloop

import (
	"context"
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// newTestLoopWithPlayer builds a GameLoop with a single player (charID 7,
// account "acc") and returns the loop plus the player's world state. No client
// connection is registered, so packet sends are no-ops.
func newTestLoopWithPlayer(t *testing.T) (*GameLoop, *registry.PlayerWorldState) {
	t.Helper()
	world := registry.NewWorldRegistry()
	char := &models.Character{
		ID:          7,
		AccountName: "acc",
		Name:        "Tester",
		Level:       1,
		MaxHP:       100,
		CurrentHP:   100,
		Position:    models.Position{X: 0, Y: 0, Z: 0},
	}
	if err := world.AddPlayer(context.Background(), char); err != nil {
		t.Fatalf("AddPlayer: %v", err)
	}
	gl := New(world, registry.NewConnectionRegistry(), 1, 1)
	player, ok := world.GetPlayer(7)
	if !ok {
		t.Fatal("player not found after AddPlayer")
	}
	return gl, player
}

func addAttackableNPC(gl *GameLoop, objectID int32, pos models.Position) *models.NpcInstance {
	npc := &models.NpcInstance{
		ObjectID:  objectID,
		Position:  pos,
		IsRunning: true,
		CurrentHP: 100,
		Template: &models.NpcTemplate{
			ID:   12345,
			Name: "TestMob",
			Type: "L2Monster",
			HP:   100,
			PDef: 10,
		},
	}
	gl.world.AddNPC(npc)
	return npc
}

// TestHandleAttackRequestDoesNotEnterCombatStance verifies that issuing an attack
// request (a click on a mob) does NOT put the player into combat stance. In L2J the
// stance begins on the first actual hit, not on the request. (l2go-7qv)
func TestHandleAttackRequestDoesNotEnterCombatStance(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	npc := addAttackableNPC(gl, 1000, models.Position{X: 0, Y: 0, Z: 0})

	gl.handleAttackRequest(CmdAttackRequest{
		AttackerCharID: 7,
		TargetObjectID: npc.ObjectID,
		AttackerPos:    player.Position,
		AccountName:    "acc",
	})

	if player.InCombat {
		t.Error("attack request must NOT enter combat stance before a hit lands")
	}
}

// TestEnterCombatStanceTransition verifies the helper sets InCombat once and is a
// no-op when the player is already in stance. (l2go-7qv)
func TestEnterCombatStanceTransition(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)

	if player.InCombat {
		t.Fatal("player should start out of combat")
	}
	gl.enterCombatStance(7)
	if !player.InCombat {
		t.Fatal("enterCombatStance must set InCombat=true")
	}
	// Idempotent: a second call while already in stance must not panic or change state.
	gl.enterCombatStance(7)
	if !player.InCombat {
		t.Error("InCombat must remain true after a repeat call")
	}
}

// TestFirstSwingEntersCombatStance verifies that the first real swing (NextAttackEvent
// with the target in range) puts the attacker into combat stance — matching L2J, where
// the weapon is drawn on doAttack(), not on the attack request. (l2go-7qv)
func TestFirstSwingEntersCombatStance(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	npc := addAttackableNPC(gl, 1000, models.Position{X: 0, Y: 0, Z: 0}) // same pos → in melee range
	gl.combatState[7] = &PlayerCombatState{
		IsAutoAttacking: true,
		TargetObjectID:  npc.ObjectID,
		AccountName:     "acc",
		LastAttackTime:  time.Now(),
	}

	(&NextAttackEvent{
		AttackerCharID: 7,
		TargetObjectID: npc.ObjectID,
	}).Execute(gl)

	if !player.InCombat {
		t.Error("first swing must enter combat stance")
	}
}

// TestTargetDeathReleasesAttacker verifies that when an attacked NPC dies, the attacker
// is released: auto-attack stops, intention resets to idle, and server-side approach
// movement halts (the basis for the StopMove that unblocks the client). (l2go-p80)
func TestTargetDeathReleasesAttacker(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	npc := addAttackableNPC(gl, 1000, models.Position{X: 100, Y: 0, Z: 0})
	gl.combatState[7] = &PlayerCombatState{
		IsAutoAttacking: true,
		TargetObjectID:  npc.ObjectID,
		AccountName:     "acc",
		LastAttackTime:  time.Now(),
	}
	gl.setIntention(7, IntentionAttack, npc.ObjectID)
	player.IsMoving = true
	player.MoveDestination = models.Position{X: 50, Y: 0, Z: 0}

	gl.stopAllAttackersOnTarget(npc.ObjectID)

	if gl.combatState[7].IsAutoAttacking {
		t.Error("auto-attack must stop when the target dies")
	}
	if st := gl.aiState[7]; st == nil || st.Intention != IntentionIdle {
		t.Errorf("intention must reset to idle on target death, got %+v", st)
	}
	if player.IsMoving {
		t.Error("server-side approach movement must halt on target death (release follow-pawn)")
	}
}

// TestNPCHitEventEntersCombatStance verifies that taking damage from an NPC puts
// the player into combat stance. (l2go-7qv)
func TestNPCHitEventEntersCombatStance(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	npc := addAttackableNPC(gl, 1000, models.Position{X: 0, Y: 0, Z: 0})

	(&NPCHitEvent{
		NPCObjectID:  npc.ObjectID,
		TargetCharID: 7,
		Damage:       5,
	}).Execute(gl)

	if !player.InCombat {
		t.Error("taking damage must enter combat stance")
	}
}
