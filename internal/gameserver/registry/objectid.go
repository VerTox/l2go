package registry

import "sync/atomic"

// npcObjectIDCounter provides unique object IDs for NPCs.
// Starts at 1_000_000 to avoid collisions with character IDs from the DB.
var npcObjectIDCounter int32 = 1_000_000

// NextNPCObjectID returns the next unique object ID for an NPC.
func NextNPCObjectID() int32 {
	return atomic.AddInt32(&npcObjectIDCounter, 1)
}
