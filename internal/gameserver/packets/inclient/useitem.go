package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// UseItem packet (opcode 0x19) - double-click on item / drag to paperdoll
// Toggle: if equipped → unequip, if in inventory → equip
type UseItem struct {
	ObjectID    int32
	CtrlPressed bool
}

// NewUseItem parses UseItem packet from payload
func NewUseItem(payload []byte) *UseItem {
	r := l2pkt.NewReader(payload)
	d, _ := r.ReadD()
	objectID := d
	d2, _ := r.ReadD()
	ctrlPressed := d2 != 0

	return &UseItem{
		ObjectID:    objectID,
		CtrlPressed: ctrlPressed,
	}
}
