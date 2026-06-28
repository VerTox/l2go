package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// StatusUpdate attribute IDs (from L2J StatusUpdate.java).
const (
	StatusLevel = 0x01
	StatusExp   = 0x02
	StatusCurHP = 0x09
	StatusMaxHP = 0x0A
	StatusCurMP = 0x0B
	StatusMaxMP = 0x0C
	StatusSP    = 0x0D
	StatusCurCP = 0x21
	StatusMaxCP = 0x22
)

// StatusAttribute represents a single attribute in a StatusUpdate packet.
type StatusAttribute struct {
	ID    int32
	Value int32
}

// BuildStatusUpdate builds the StatusUpdate packet (0x0E).
// Sent to the client to update HP/MP/CP bars for a targeted object.
func BuildStatusUpdate(objectID int32, attrs []StatusAttribute) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x18)
	w.WriteD(objectID)
	w.WriteD(int32(len(attrs)))
	for _, attr := range attrs {
		w.WriteD(attr.ID)
		w.WriteD(attr.Value)
	}
	return w.Bytes()
}
