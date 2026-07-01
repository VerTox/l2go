package outclient

import "testing"

func TestTeleportToLocation(t *testing.T) {
	checkGolden(t, "teleporttolocation", BuildTeleportToLocation(0x10203040, 100, 200, -300, 16384))
}

func TestRevive(t *testing.T) {
	checkGolden(t, "revive", BuildRevive(0x10203040))
}
