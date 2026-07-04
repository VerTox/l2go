package registry

import "testing"

// TestPackRegion_UniqueAcrossQuadrants verifies the int64 region key is collision-
// free for distinct (rx,ry) cells, including negative coordinates (two's-complement
// low-32 packing). A collision would merge two grid cells and corrupt visibility.
func TestPackRegion_Unique(t *testing.T) {
	seen := make(map[int64][2]int)
	for rx := -200; rx <= 200; rx++ {
		for ry := -200; ry <= 200; ry++ {
			k := packRegion(rx, ry)
			if prev, ok := seen[k]; ok {
				t.Fatalf("collision: (%d,%d) and (%d,%d) both pack to %d", rx, ry, prev[0], prev[1], k)
			}
			seen[k] = [2]int{rx, ry}
		}
	}
}

// TestGetRegionKey_ConsistentWithNearby checks a point's own cell key is among the
// keys getNearbyRegions returns for a small radius around it — the two must agree
// or an object would index into a cell the range query never scans.
func TestGetRegionKey_ConsistentWithNearby(t *testing.T) {
	wr := NewWorldRegistry()
	points := [][2]int{{0, 0}, {1500, -2500}, {-999, 999}, {123456, -654321}}
	for _, p := range points {
		self := wr.getRegionKey(p[0], p[1])
		found := false
		for _, k := range wr.getNearbyRegions(p[0], p[1], 100) {
			if k == self {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("point (%d,%d): own region key %d not in getNearbyRegions", p[0], p[1], self)
		}
	}
}
