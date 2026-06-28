package inclient

import (
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// ProtocolVersion carries the client's protocol version.
type ProtocolVersion struct {
	Version uint32
}

func NewProtocolVersion(data []byte) *ProtocolVersion {
	r := l2pkt.NewReader(data)
	d, _ := r.ReadD()
	return &ProtocolVersion{Version: uint32(d)}
}
