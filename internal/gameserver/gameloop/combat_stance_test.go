package gameloop

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

// recordingConn captures the raw packet payloads (opcode + body, length header
// stripped) written to a client connection, for asserting broadcast behaviour.
type recordingConn struct {
	mu      sync.Mutex
	packets [][]byte
}

// newRecordingConn returns a *client.ClientConn whose writes are captured by the
// returned recordingConn. Uses net.Pipe, so a reader goroutine drains writes.
func newRecordingConn(t *testing.T) (*client.ClientConn, *recordingConn) {
	t.Helper()
	srv, cli := net.Pipe()
	rc := &recordingConn{}
	go rc.readLoop(cli)
	t.Cleanup(func() { _ = srv.Close(); _ = cli.Close() })
	return client.NewClientConn(srv), rc
}

func (rc *recordingConn) readLoop(r net.Conn) {
	for {
		header := make([]byte, 2)
		if _, err := io.ReadFull(r, header); err != nil {
			return
		}
		total := int(binary.LittleEndian.Uint16(header))
		if total < 2 {
			return
		}
		body := make([]byte, total-2)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		rc.mu.Lock()
		rc.packets = append(rc.packets, body)
		rc.mu.Unlock()
	}
}

// contains reports whether an exact packet payload was written.
func (rc *recordingConn) contains(want []byte) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for _, p := range rc.packets {
		if bytes.Equal(p, want) {
			return true
		}
	}
	return false
}

// eventually polls f up to ~1s to tolerate the async pipe reader goroutine.
func eventually(f func() bool) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return f()
}

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

// TestCombatStanceTimeoutBroadcastsToNearby verifies that when a player's combat stance
// expires (15s timeout), the AutoAttackStop is broadcast to ALL nearby players — not just
// the owner — so the weapon-drawn stance is sheathed on everyone's client. This is the fix
// for l2go-k0f: the set path (enterCombatStance) broadcasts AutoAttackStart to the
// knownlist, but the clear path used to notify only the owner, leaving neighbours stuck in
// the stale stance until their own timeout. (l2go-k0f)
func TestCombatStanceTimeoutBroadcastsToNearby(t *testing.T) {
	world := registry.NewWorldRegistry()
	owner := &models.Character{
		ID: 7, AccountName: "owner", Name: "Owner", Level: 1,
		MaxHP: 100, CurrentHP: 100, Position: models.Position{X: 0, Y: 0, Z: 0},
	}
	neighbor := &models.Character{
		ID: 8, AccountName: "neighbor", Name: "Neighbor", Level: 1,
		MaxHP: 100, CurrentHP: 100, Position: models.Position{X: 10, Y: 10, Z: 0},
	}
	if err := world.AddPlayer(context.Background(), owner); err != nil {
		t.Fatalf("AddPlayer owner: %v", err)
	}
	if err := world.AddPlayer(context.Background(), neighbor); err != nil {
		t.Fatalf("AddPlayer neighbor: %v", err)
	}

	conns := registry.NewConnectionRegistry()
	ownerConn, ownerRec := newRecordingConn(t)
	neighborConn, neighborRec := newRecordingConn(t)
	conns.Register("owner", ownerConn)
	conns.Register("neighbor", neighborConn)

	gl := New(world, conns, 1, 1)

	// Put the owner into combat stance with an expired last-attack time so the timeout fires.
	world.SetPlayerCombatState(7, true)
	gl.combatState[7] = &PlayerCombatState{
		IsAutoAttacking: false,
		TargetObjectID:  0,
		AccountName:     "owner",
		LastAttackTime:  time.Now().Add(-20 * time.Second),
	}

	(&CombatStanceTimeoutEvent{At: time.Now(), CharID: 7}).Execute(gl)

	wantStop := outclient.BuildAutoAttackStop(7)

	if !eventually(func() bool { return neighborRec.contains(wantStop) }) {
		t.Error("combat-stance timeout must broadcast AutoAttackStop to nearby players (neighbor did not receive it)")
	}
	if !eventually(func() bool { return ownerRec.contains(wantStop) }) {
		t.Error("combat-stance timeout must also sheath the stance on the owner's client")
	}
	if _, ok := gl.combatState[7]; ok {
		t.Error("combat state must be deleted after timeout")
	}
}
