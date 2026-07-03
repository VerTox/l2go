package registry

import "testing"

// Charge stores the weapon grade alongside the charge so the attack path can
// bake the soulshot grade into the Attack visual flag (l2go-77a).
func TestChargedShot_GradeRoundTrip(t *testing.T) {
	r := NewChargedShotRegistry()
	const weapon int32 = 4242

	r.Charge(weapon, ShotSoulshot, int(GradeS))
	if !r.IsCharged(weapon, ShotSoulshot) {
		t.Fatal("weapon should be charged after Charge")
	}
	if got := r.ChargedGrade(weapon, ShotSoulshot); got != int(GradeS) {
		t.Fatalf("ChargedGrade = %d, want %d", got, int(GradeS))
	}

	r.Clear(weapon)
	if r.IsCharged(weapon, ShotSoulshot) {
		t.Fatal("weapon should not be charged after Clear")
	}
	if got := r.ChargedGrade(weapon, ShotSoulshot); got != 0 {
		t.Fatalf("ChargedGrade after Clear = %d, want 0", got)
	}
}

// SetCharged remains the grade-less toggle used to spend a charge (charged=false)
// and by tests that pre-charge without caring about grade.
func TestChargedShot_SetChargedTogglesGradeZero(t *testing.T) {
	r := NewChargedShotRegistry()
	const weapon int32 = 7

	r.SetCharged(weapon, ShotSoulshot, true)
	if !r.IsCharged(weapon, ShotSoulshot) {
		t.Fatal("SetCharged(true) should charge")
	}
	if got := r.ChargedGrade(weapon, ShotSoulshot); got != 0 {
		t.Fatalf("grade-less charge grade = %d, want 0", got)
	}

	r.SetCharged(weapon, ShotSoulshot, false)
	if r.IsCharged(weapon, ShotSoulshot) {
		t.Fatal("SetCharged(false) should spend the charge")
	}
}

// Spending a graded charge (SetCharged false) clears the grade too.
func TestChargedShot_SpendClearsGrade(t *testing.T) {
	r := NewChargedShotRegistry()
	const weapon int32 = 99

	r.Charge(weapon, ShotSoulshot, int(GradeA))
	r.SetCharged(weapon, ShotSoulshot, false)

	if r.IsCharged(weapon, ShotSoulshot) {
		t.Fatal("charge should be spent")
	}
	if got := r.ChargedGrade(weapon, ShotSoulshot); got != 0 {
		t.Fatalf("grade after spend = %d, want 0", got)
	}
}
