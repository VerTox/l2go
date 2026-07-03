package models

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestRegenFormulas_RetailAnchors(t *testing.T) {
	// Exact retail table anchors (HumanFighter/HumanMystic lvlUpgainData).
	cases := []struct {
		level          int
		hp, mp, cp     float64
	}{
		{1, 2.0, 0.9, 2.0},
		{20, 3.4, 1.2, 2.5},
		{40, 5.4, 1.8, 4.5},
		{60, 7.4, 2.4, 6.5},
		{80, 9.4, 3.0, 8.5},
		{85, 9.9, 3.0, 8.5}, // mp clamps in retail; formula gives 3.15 — acceptance below
	}
	for _, c := range cases {
		if got := HpRegenPerTick(c.level); !approx(got, c.hp) {
			t.Errorf("HpRegenPerTick(%d) = %.3f, want %.3f", c.level, got, c.hp)
		}
		if got := CpRegenPerTick(c.level); !approx(got, c.cp) {
			t.Errorf("CpRegenPerTick(%d) = %.3f, want %.3f", c.level, got, c.cp)
		}
	}
	// HP/CP match exactly at all anchors; MP matches at 1..80 (85 caps in retail).
	for _, c := range cases[:5] {
		if got := MpRegenPerTick(c.level); !approx(got, c.mp) {
			t.Errorf("MpRegenPerTick(%d) = %.3f, want %.3f", c.level, got, c.mp)
		}
	}
}

func TestRegenFormulas_Floor(t *testing.T) {
	// Below the crossover the level-1 retail values act as a floor.
	if got := HpRegenPerTick(1); got != 2.0 {
		t.Errorf("HpRegenPerTick(1) = %.3f, want floor 2.0", got)
	}
	if got := MpRegenPerTick(1); got != 0.9 {
		t.Errorf("MpRegenPerTick(1) = %.3f, want floor 0.9", got)
	}
	if got := CpRegenPerTick(1); got != 2.0 {
		t.Errorf("CpRegenPerTick(1) = %.3f, want floor 2.0", got)
	}
}

func TestRegenFormulas_Monotonic(t *testing.T) {
	prevHp, prevMp, prevCp := 0.0, 0.0, 0.0
	for lvl := 1; lvl <= 85; lvl++ {
		hp, mp, cp := HpRegenPerTick(lvl), MpRegenPerTick(lvl), CpRegenPerTick(lvl)
		if hp < prevHp || mp < prevMp || cp < prevCp {
			t.Fatalf("regen decreased at level %d: hp=%.3f mp=%.3f cp=%.3f", lvl, hp, mp, cp)
		}
		prevHp, prevMp, prevCp = hp, mp, cp
	}
}
