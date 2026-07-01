package registry

import (
	"os"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// mapRegionDataDir finds the L2J map-region XML directory relative to this package.
func mapRegionDataDir(t *testing.T) string {
	t.Helper()
	for _, p := range []string{
		"../../../references/data/mapregion",
		"../../../../references/data/mapregion",
	} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	t.Skip("map-region reference data not found")
	return ""
}

func TestMapRegionRespawnPoint(t *testing.T) {
	r := NewMapRegionRegistry()
	if err := r.LoadFromDirectory(mapRegionDataDir(t)); err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}
	if r.Count() == 0 {
		t.Fatal("no regions loaded")
	}

	// Talking Island starting position → talking_island_town first respawn point.
	got, ok := r.GetRespawnPoint(-84318, 244579)
	if !ok {
		t.Fatal("no respawn point resolved for Talking Island start position")
	}
	want := models.Position{X: -83990, Y: 243336, Z: -3700}
	if got != want {
		t.Errorf("respawn point = %+v, want %+v", got, want)
	}
}

func TestMapRegionFallbackToDefault(t *testing.T) {
	r := NewMapRegionRegistry()
	if err := r.LoadFromDirectory(mapRegionDataDir(t)); err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}
	// A coordinate far outside any region tile must fall back to the default town.
	got, ok := r.GetRespawnPoint(1<<28, 1<<28)
	if !ok {
		t.Fatal("expected fallback respawn point")
	}
	if got == (models.Position{}) {
		t.Error("fallback respawn point must be non-zero")
	}
}
