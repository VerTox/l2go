package usecase

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// TestGetCombatBaseStatsByClass_MatchesTemplates verifies the memoized map returns
// the same combat stats the linear template scan did, and falls back to fighter
// stats for a class absent from the base templates. (l2go-795)
func TestGetCombatBaseStatsByClass_MatchesTemplates(t *testing.T) {
	// Known base class → its template's combat stats (mystic differs from fighter,
	// so a correct lookup is observable).
	fighter := GetCombatBaseStatsByClass(int(models.ClassHumanFighter))
	mystic := GetCombatBaseStatsByClass(int(models.ClassHumanMystic))
	if fighter == mystic {
		t.Fatal("fighter and mystic combat stats should differ — lookup is not class-specific")
	}

	// Cross-check against a direct scan of the templates.
	for _, tpl := range getDefaultCharacterTemplates() {
		got := GetCombatBaseStatsByClass(tpl.ClassID)
		if got != tpl.BaseStats.Combat {
			t.Errorf("class %d: map lookup %+v != template %+v", tpl.ClassID, got, tpl.BaseStats.Combat)
		}
	}

	// Unknown class → fighter fallback (unchanged behavior).
	if got := GetCombatBaseStatsByClass(9999); got != fighterCombatStats() {
		t.Errorf("unknown class fallback = %+v, want fighter stats", got)
	}
}

// BenchmarkGetCombatBaseStatsByClass shows the memoized lookup is a cheap, zero-
// allocation map read. Before l2go-795 each call rebuilt the whole template slice
// (with every class's StartingItems) and scanned it linearly.
func BenchmarkGetCombatBaseStatsByClass(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GetCombatBaseStatsByClass(int(models.ClassHumanFighter))
	}
}

// BenchmarkComputeCharacterStats measures the full stat computation on the hot path
// (called from movement/combat/cast/CharInfo). Post-l2go-795 the class lookup no
// longer rebuilds templates, so this is dominated by cheap arithmetic.
func BenchmarkComputeCharacterStats(b *testing.B) {
	char := &models.Character{
		ClassID: int(models.ClassHumanFighter), Level: 40,
		BaseSTR: 40, BaseDEX: 30, BaseCON: 43, BaseINT: 25, BaseWIT: 11, BaseMEN: 25,
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ComputeCharacterStats(char)
	}
}
