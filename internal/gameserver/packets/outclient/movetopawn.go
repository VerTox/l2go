package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildMoveToPawn builds the MoveToPawn packet (0x72).
// Sent to the client to command the player character to move/face toward a target.
// In L2J this is sent before NPC dialogue to transition the client state properly.
func BuildMoveToPawn(charObjID, targetObjID int32, distance int32, px, py, pz, tx, ty, tz int) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x72)
	w.WriteD(charObjID)
	w.WriteD(targetObjID)
	w.WriteD(distance)
	w.WriteD(int32(px))
	w.WriteD(int32(py))
	w.WriteD(int32(pz))
	w.WriteD(int32(tx))
	w.WriteD(int32(ty))
	w.WriteD(int32(tz))
	return w.Bytes()
}
