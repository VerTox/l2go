package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildActionFailed builds the ActionFailed packet (0x1F, per L2J HF).
// Sent to unblock the client after an action that cannot be completed — without it
// the client stays in an action-pending lock (can't move or retarget) until the
// target is cancelled. NOTE: 0x25 (the previous, wrong value) is AutoAttackStart, so
// every ActionFailed was going out as a malformed AutoAttackStart and never released
// the client. (l2go-p80)
func BuildActionFailed() []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x1F)
	return w.Bytes()
}
