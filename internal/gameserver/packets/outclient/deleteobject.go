package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// DeleteObject packet (0x08) - removes an object from client's world
// Used when players logout, die, go out of visibility range, etc.
type DeleteObject struct {
	ObjectID int32
}

// NewDeleteObject creates a new DeleteObject packet
func NewDeleteObject(objectID int32) *DeleteObject {
	return &DeleteObject{
		ObjectID: objectID,
	}
}

// BuildDeleteObject creates DeleteObject packet data using pkg/l2pkt
func BuildDeleteObject(objectID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x08) // DeleteObject opcode
	w.WriteD(objectID)
	return w.Bytes()
}

// GetData returns the packet data bytes
func (p *DeleteObject) GetData() []byte {
	return BuildDeleteObject(p.ObjectID)
}