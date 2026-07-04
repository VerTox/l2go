package registry

import "testing"

func TestCanTeach(t *testing.T) {
	// Load a minimal category: HUMAN_FALL_CLASS contains class 0.
	if err := GetCategoryRegistry().load([]byte(`<list>
<category name="HUMAN_FALL_CLASS"><id>0</id></category>
</list>`)); err != nil {
		t.Fatal(err)
	}

	const fighterNPC = 30010 // in fighterCoachNPCs

	if !IsTrainer(fighterNPC) {
		t.Fatal("30010 must be a trainer")
	}
	if IsTrainer(99999) {
		t.Fatal("99999 must not be a trainer")
	}
	// Human (race 0) fighter class 0 → fighter coach teaches.
	if !CanTeach(fighterNPC, 0, 0, 0) {
		t.Fatal("fighter coach must teach human fighter class 0")
	}
	// Elf (race 1) → no ELF_FALL_CLASS category loaded → refused.
	if CanTeach(fighterNPC, 1, 0, 0) {
		t.Fatal("fighter coach must not teach elf (no matching category)")
	}
	// Non-trainer npc → false.
	if CanTeach(99999, 0, 0, 0) {
		t.Fatal("non-trainer must not teach")
	}
}
