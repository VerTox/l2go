package gameloop

import "testing"

// attackReach mirrors L2J L2CharacterAI.maybeMoveToPawn: the center-to-center
// distance at which an attack connects is the base weapon/skill range plus both
// actors' collision radii. The same value is used for the MoveToPawn offset and
// the hit-range check, so the client stops exactly where the server expects.
func TestAttackReach(t *testing.T) {
	tests := []struct {
		name              string
		baseRange         int
		attackerCollision float64
		targetCollision   float64
		want              int
	}{
		// Unarmed melee (40) vs a small mob: 40 + 9 + 15 = 64.
		{name: "small mob", baseRange: 40, attackerCollision: 9, targetCollision: 15, want: 64},
		// Same melee vs a large mob: 40 + 9 + 40 = 89. The old hardcoded offset 60
		// landed the client at 60+collisions > 90, which is why big mobs "didn't reach".
		{name: "large mob", baseRange: 40, attackerCollision: 9, targetCollision: 40, want: 89},
		// Bow-like long base range still just adds collisions.
		{name: "ranged base", baseRange: 500, attackerCollision: 9, targetCollision: 20, want: 529},
		// Collision radii are floored (truncated) to int, matching L2J int math.
		{name: "fractional collisions floored", baseRange: 40, attackerCollision: 9.8, targetCollision: 15.7, want: 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attackReach(tt.baseRange, tt.attackerCollision, tt.targetCollision)
			if got != tt.want {
				t.Errorf("attackReach(%d, %g, %g) = %d, want %d",
					tt.baseRange, tt.attackerCollision, tt.targetCollision, got, tt.want)
			}
		})
	}
}
