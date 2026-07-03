package data

import (
	"math"
	"testing"
)

// CalcNPCBaseExp mirrors L2J L2Npc.getExpReward before the server rate:
// exp = level² × expRate (expRate is the datapack <acquire> coefficient).
func TestCalcNPCBaseExp(t *testing.T) {
	if got := CalcNPCBaseExp(10, 2.0); got != 200 {
		t.Errorf("CalcNPCBaseExp(10, 2.0) = %d, want 200", got)
	}
	// level 81, expRate 3.84310575 (Maluk Maiden) → 6561 × 3.843... ≈ 25214
	rate := 3.84310575
	want := int64(float64(6561) * rate)
	if got := CalcNPCBaseExp(81, rate); got != want {
		t.Errorf("CalcNPCBaseExp(81, %v) = %d, want %d", rate, got, want)
	}
	// No <acquire> (expRate 0) → 0 exp.
	if got := CalcNPCBaseExp(50, 0); got != 0 {
		t.Errorf("CalcNPCBaseExp(50, 0) = %d, want 0", got)
	}
}

// LevelPenalty is asymmetric (L2J calculateExpAndSp): penalty only when the player
// is more than 5 levels ABOVE the mob; at/below the mob it is full exp, no 1% floor.
func TestLevelPenalty_Asymmetric(t *testing.T) {
	// Player at or below the mob (or within +5): no penalty.
	for _, pl := range []int{60, 75, 80, 85} { // npc level 80, diff <= 5
		if got := LevelPenalty(pl, 80); got != 1.0 {
			t.Errorf("LevelPenalty(%d, 80) = %v, want 1.0", pl, got)
		}
	}
	// Player >5 above: (5/6)^(diff-5).
	if got := LevelPenalty(86, 80); math.Abs(got-5.0/6.0) > 1e-9 { // diff 6
		t.Errorf("LevelPenalty(86, 80) = %v, want %v", got, 5.0/6.0)
	}
	if got := LevelPenalty(90, 80); math.Abs(got-math.Pow(5.0/6.0, 5)) > 1e-9 { // diff 10
		t.Errorf("LevelPenalty(90, 80) = %v, want %v", got, math.Pow(5.0/6.0, 5))
	}
	// No 1% floor: a huge over-level keeps shrinking below 0.01.
	if got := LevelPenalty(130, 80); got >= 0.01 { // diff 50 → (5/6)^45
		t.Errorf("LevelPenalty(130, 80) = %v, want < 0.01 (no floor)", got)
	}
}
