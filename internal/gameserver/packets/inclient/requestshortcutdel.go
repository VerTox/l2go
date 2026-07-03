package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// ShortcutDeleteRequest is RequestShortCutDel (0x3f): the client informs the
// server a shortcut was removed from the quick bar. A single packed slot is sent
// (page*12 + slot); no server confirmation is expected (L2J parity).
type ShortcutDeleteRequest struct {
	Slot int32
	Page int32
}

func (p *ShortcutDeleteRequest) Read(r *l2pkt.Reader) bool {
	slot, err := r.ReadD()
	if err != nil {
		return false
	}
	p.Slot = slot % 12
	p.Page = slot / 12
	return true
}
