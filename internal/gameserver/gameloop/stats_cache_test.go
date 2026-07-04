package gameloop

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// TestComputePlayerStats_CachesAndInvalidates verifies computePlayerStats memoizes
// the result and that stat changes invalidate it (l2go-gur): a hit returns the
// cached value even if the underlying Character mutates without an invalidate,
// while RebuildStatMods / a level-up force a fresh recompute.
func TestComputePlayerStats_CachesAndInvalidates(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	player.Character.ClassID = int(models.ClassHumanFighter)
	player.Character.Level = 10
	player.InvalidateStats() // start from a clean miss

	first := gl.computePlayerStats(player)

	// Mutate StatMods directly WITHOUT invalidating: a cache hit must still return
	// the previously computed value (proves the cache is actually consulted).
	player.Character.StatMods = []models.StatModifier{{Stat: models.StatPAtk, Op: "add", Val: 1000}}
	if got := gl.computePlayerStats(player); got.PAtk != first.PAtk {
		t.Errorf("cache not used: PAtk changed to %v without invalidation (was %v)", got.PAtk, first.PAtk)
	}

	// RebuildStatMods invalidates → the recompute must reflect the +1000 pAtk gear.
	player.EquipMods = []models.StatModifier{{Stat: models.StatPAtk, Op: "add", Val: 1000}}
	player.RebuildStatMods()
	afterEquip := gl.computePlayerStats(player)
	if afterEquip.PAtk != first.PAtk+1000 {
		t.Errorf("after equip+invalidate: PAtk = %v, want %v (+1000)", afterEquip.PAtk, first.PAtk+1000)
	}

	// InvalidateStats (the path applyLevelUp uses) must force a recompute that picks
	// up subsequent changes — mutate StatMods directly, invalidate, and confirm the
	// next call reflects it.
	player.EquipMods = nil
	player.RebuildStatMods() // back to base, invalidates
	base := gl.computePlayerStats(player)
	player.Character.StatMods = []models.StatModifier{{Stat: models.StatPAtk, Op: "add", Val: 777}}
	player.InvalidateStats() // as applyLevelUp does
	after := gl.computePlayerStats(player)
	if after.PAtk != base.PAtk+777 {
		t.Errorf("InvalidateStats did not force a recompute: PAtk = %v, want %v", after.PAtk, base.PAtk+777)
	}
}

// TestComputePlayerStats_MatchesUncached pins the cached result to a direct
// (uncached) computation — memoization must not change the value.
func TestComputePlayerStats_MatchesUncached(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	player.Character.ClassID = int(models.ClassHumanFighter)
	player.Character.Level = 25
	player.EquipMods = []models.StatModifier{{Stat: models.StatPAtk, Op: "add", Val: 42}}
	player.RebuildStatMods()

	cached := gl.computePlayerStats(player)
	direct := models.ComputedStats{}
	// Recompute independently via the same path a fresh miss would take.
	player.InvalidateStats()
	direct = gl.computePlayerStats(player)
	if cached != direct {
		t.Errorf("cached stats %+v differ from recomputed %+v", cached, direct)
	}
}
