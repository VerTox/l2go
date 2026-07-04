package gameloop

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// The per-second sweeps (serviceBuffs, expirePvPFlags) iterate a small loop-owned
// membership set instead of scanning all N online every tick (l2go-t2q). These
// tests pin the set maintenance: a player is tracked exactly while it carries an
// active effect / PvP flag, and untracked on expiry, toggle-off, and disconnect.

func TestServiceBuffs_TracksAndUntracksBuffedPlayer(t *testing.T) {
	gl, player := loopWithBuffSkill(t, windWalkXML)

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1204})
	(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)

	if _, ok := gl.buffedPlayers[7]; !ok {
		t.Fatal("player not tracked in buffedPlayers after gaining a buff")
	}

	// Force expiry and service: the buff drops and so must the membership.
	player.Effects.Buffs()[0].ExpiresAt = time.Now().Add(-time.Second)
	gl.serviceBuffs()

	if player.Effects.Len() != 0 {
		t.Fatalf("buff not expired: Len = %d", player.Effects.Len())
	}
	if _, ok := gl.buffedPlayers[7]; ok {
		t.Error("player still tracked in buffedPlayers after last buff expired")
	}
}

func TestServiceBuffs_UntracksOnToggleOff(t *testing.T) {
	xml := `<list><skill id="1204" levels="1" name="Toggle">
		<set name="operateType" val="T" /><set name="targetType" val="SELF" />
		<set name="abnormalType" val="NONE" />
		<effects><effect name="Buff"><add stat="pAtk" val="10" /></effect></effects>
	</skill></list>`
	gl, player := loopWithBuffSkill(t, xml)

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1204})
	if player.Casting != nil {
		(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)
	}
	if _, ok := gl.buffedPlayers[7]; !ok {
		t.Fatal("player not tracked after toggle on")
	}

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1204}) // recast = toggle off
	if _, ok := gl.buffedPlayers[7]; ok {
		t.Error("player still tracked after toggle off")
	}
}

func TestExpirePvPFlags_TracksAndUntracks(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)

	gl.setPvPFlag(player)
	if _, ok := gl.flaggedPlayers[7]; !ok {
		t.Fatal("player not tracked in flaggedPlayers after setPvPFlag")
	}

	// Not yet expired: the sweep keeps it.
	gl.expirePvPFlags()
	if _, ok := gl.flaggedPlayers[7]; !ok {
		t.Fatal("flag dropped before it expired")
	}
	if player.PvPFlagUntil.IsZero() {
		t.Fatal("flag cleared before it expired")
	}

	// Force expiry: the sweep clears and untracks.
	player.PvPFlagUntil = time.Now().Add(-time.Second)
	gl.expirePvPFlags()
	if !player.PvPFlagUntil.IsZero() {
		t.Error("flag not cleared after expiry")
	}
	if _, ok := gl.flaggedPlayers[7]; ok {
		t.Error("player still tracked after flag expired")
	}
}

// TestExpirePvPFlags_OnlyTracksFlagged proves the sweep is scoped to the flagged
// subset, not all online: with three players and one flag, the set holds exactly
// one entry and the sweep touches only it.
func TestExpirePvPFlags_OnlyTracksFlagged(t *testing.T) {
	gl, _ := newTestLoopWithPlayer(t) // charID 7
	addPlayer(t, gl, 8, "acc8", models.Position{})
	addPlayer(t, gl, 9, "acc9", models.Position{})

	p8, _ := gl.world.GetPlayer(8)
	gl.setPvPFlag(p8)

	if len(gl.flaggedPlayers) != 1 {
		t.Fatalf("flaggedPlayers = %d entries, want exactly 1 (only the flagged player)", len(gl.flaggedPlayers))
	}
	if _, ok := gl.flaggedPlayers[8]; !ok {
		t.Fatal("the flagged player (8) is not the tracked one")
	}

	// After the one flag expires the set empties, so the sweep becomes a genuine
	// no-op over zero entries regardless of how many players are online.
	p8.PvPFlagUntil = time.Now().Add(-time.Second)
	gl.expirePvPFlags()
	if len(gl.flaggedPlayers) != 0 {
		t.Fatalf("flaggedPlayers should be empty after clear, got %d", len(gl.flaggedPlayers))
	}
}

func TestDisconnect_UntracksSweepMembership(t *testing.T) {
	gl, player := loopWithBuffSkill(t, windWalkXML)

	// Give the player both a buff and a PvP flag.
	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1204})
	(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)
	gl.setPvPFlag(player)

	if _, ok := gl.buffedPlayers[7]; !ok {
		t.Fatal("precondition: player should be in buffedPlayers")
	}
	if _, ok := gl.flaggedPlayers[7]; !ok {
		t.Fatal("precondition: player should be in flaggedPlayers")
	}

	gl.handlePlayerDisconnected(CmdPlayerDisconnected{CharID: 7})

	if _, ok := gl.buffedPlayers[7]; ok {
		t.Error("player still in buffedPlayers after disconnect")
	}
	if _, ok := gl.flaggedPlayers[7]; ok {
		t.Error("player still in flaggedPlayers after disconnect")
	}
}

// BenchmarkServiceBuffs_Idle measures the per-second buff+pvp sweep with N idle
// players, none buffed or flagged (the common case). Post-l2go-t2q it iterates the
// empty membership sets, so ns/op stays flat as N grows instead of scanning all N.
func BenchmarkServiceBuffs_Idle(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			world := registry.NewWorldRegistry()
			for i := 0; i < n; i++ {
				_ = world.AddPlayer(context.Background(), &models.Character{
					ID: int32(i + 1), MaxHP: 100, CurrentHP: 100,
				})
			}
			gl := New(world, registry.NewConnectionRegistry(), 1, 1)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				gl.serviceBuffs() // also runs expirePvPFlags
			}
		})
	}
}
