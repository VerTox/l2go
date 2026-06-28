package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// RequestUnEquipItem packet (opcode 0x16) - drag item off paperdoll slot
// Sends the body part bitmask of the slot being cleared
type RequestUnEquipItem struct {
	SlotBitmask int32
}

// NewRequestUnEquipItem parses RequestUnEquipItem packet from payload
func NewRequestUnEquipItem(payload []byte) *RequestUnEquipItem {
	r := l2pkt.NewReader(payload)
	d, _ := r.ReadD()
	slotBitmask := d

	return &RequestUnEquipItem{
		SlotBitmask: slotBitmask,
	}
}
