package registry

// Visibility radii shared across the whole game server (loop, handlers, use cases)
// so players and NPCs appear/disappear at one consistent distance. Kept here in the
// registry because every layer imports it without an import cycle.
//
// Values mirror L2J High Five PcKnownList: an object is spawned to a client within
// the watch radius and only forgotten once it passes the larger forget radius. The
// gap between them is hysteresis that prevents spawn/despawn flicker at the boundary.
// (L2J additionally scales the watch radius 4500→2300 by crowd size; we use the
// representative open-field value.)
const (
	// VisibilityWatchRadius is the distance at which an object is spawned to a client
	// (CharInfo / NpcInfo).
	VisibilityWatchRadius = 3400
	// VisibilityForgetRadius is the distance at which a known object is despawned
	// (DeleteObject). Equals watch + 500, matching L2J.
	VisibilityForgetRadius = VisibilityWatchRadius + 500 // 3900
)
