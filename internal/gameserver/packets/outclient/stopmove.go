package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildStopMove builds the StopMove packet (0x47). Tells the client that the given
// object has stopped at the specified position/heading. On a target's death L2J
// broadcasts this for each attacker (AbstractAI.clientStopMoving) so the client
// leaves "follow pawn" mode and becomes movable again — without it the client stays
// locked onto the dead target and ignores ground-move clicks until ESC.
// Mirrors L2J StopMove.writeImpl: opcode + objectId + x + y + z + heading.
func BuildStopMove(objectID, x, y, z, heading int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x47) // StopMove opcode
	w.WriteD(objectID)
	w.WriteD(x)
	w.WriteD(y)
	w.WriteD(z)
	w.WriteD(heading)
	return w.Bytes()
}
