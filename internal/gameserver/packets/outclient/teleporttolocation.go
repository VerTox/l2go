package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildTeleportToLocation builds the TeleportToLocation packet (0x22, per L2J HF).
// Sent to the teleporting character and everyone who sees it, so the client unloads
// the old surroundings and moves the character to the new position. Mirrors L2J
// TeleportToLocation.writeImpl: opcode + objectId + x + y + z + isValidation(0) + heading.
func BuildTeleportToLocation(objectID, x, y, z, heading int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x22) // TeleportToLocation opcode
	w.WriteD(objectID)
	w.WriteD(x)
	w.WriteD(y)
	w.WriteD(z)
	w.WriteD(0) // isValidation
	w.WriteD(heading)
	return w.Bytes()
}
