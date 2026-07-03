package gameloop

import "testing"

// soulshotPAtk doubles pAtk when the weapon holds a soulshot charge, mirroring
// L2J Formulas.calcPhysDam ssboost (applied to pAtk before defence/crit/variance).
func TestSoulshotPAtk(t *testing.T) {
	if got := soulshotPAtk(100, false); got != 100 {
		t.Errorf("uncharged pAtk = %d, want 100", got)
	}
	if got := soulshotPAtk(100, true); got != 200 {
		t.Errorf("charged pAtk = %d, want 200 (ssboost x2)", got)
	}
	if got := soulshotPAtk(0, true); got != 0 {
		t.Errorf("charged zero pAtk = %d, want 0", got)
	}
}
