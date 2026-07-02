package gameloop

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// TestNPCRetaliatesAgainstTopHateNotLastHitter verifies that when an NPC is hit it
// retaliates against the top-hate attacker from its hate list (L2J getMostHated),
// NOT merely whoever landed the most recent blow. (l2go-z3z)
func TestNPCRetaliatesAgainstTopHateNotLastHitter(t *testing.T) {
	gl, _ := newTestLoopWithPlayer(t)
	npc := addAttackableNPC(gl, 1000, models.Position{X: 0, Y: 0, Z: 0})

	// charID 7 built up large hate earlier (e.g. a tank holding aggro).
	hl := NewHateList()
	hl.AddHate(7, 500)
	gl.npcHateLists[npc.ObjectID] = hl

	// charID 8 lands the most recent hit for a small amount of damage.
	(&HitEvent{
		AttackerCharID: 8,
		TargetObjectID: npc.ObjectID,
		Damage:         5,
	}).Execute(gl)

	ncs, ok := gl.npcCombatState[npc.ObjectID]
	if !ok || !ncs.IsAttacking {
		t.Fatal("NPC must retaliate after being hit")
	}
	if ncs.TargetCharID != 7 {
		t.Errorf("NPC must retaliate against the top-hate attacker (7), got %d (last hitter was 8)", ncs.TargetCharID)
	}
}

// TestNPCRetaliationFallsBackWhenNoHate verifies the empty/zero hate-list guard: a hit
// that yields no hate (GetTopAttacker returns 0 on an all-zero list, mirroring L2J
// returning null for an empty aggro list) must still retaliate against the attacker,
// never against invalid charID 0. (l2go-z3z)
func TestNPCRetaliationFallsBackWhenNoHate(t *testing.T) {
	gl, _ := newTestLoopWithPlayer(t)
	npc := addAttackableNPC(gl, 1000, models.Position{X: 0, Y: 0, Z: 0})

	(&HitEvent{
		AttackerCharID: 9,
		TargetObjectID: npc.ObjectID,
		Damage:         0,
	}).Execute(gl)

	ncs, ok := gl.npcCombatState[npc.ObjectID]
	if !ok || !ncs.IsAttacking {
		t.Fatal("NPC must retaliate even when the hit dealt no hate")
	}
	if ncs.TargetCharID != 9 {
		t.Errorf("retaliation must fall back to the attacker (9), got %d", ncs.TargetCharID)
	}
}
