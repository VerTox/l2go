package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildActionFailed builds the ActionFailed packet (0x25).
// Sent to unblock the client after an action that cannot be completed.
func BuildActionFailed() []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x25)
	return w.Bytes()
}
