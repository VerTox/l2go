package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// RequestEnchantItem packet (opcode 0x5f) — the client's answer to
// ChooseInventoryItem: it carries the object id of the item the player picked to
// enchant with the previously-armed scroll, plus an optional support-item id.
// Mirrors L2J HF clientpackets/RequestEnchantItem.java (readD objectId, readD supportId).
type RequestEnchantItem struct {
	ObjectID  int32 // target item to enchant
	SupportID int32 // optional support item (0 = none); support items are not yet handled
}

// NewRequestEnchantItem parses a RequestEnchantItem packet payload.
func NewRequestEnchantItem(payload []byte) *RequestEnchantItem {
	r := l2pkt.NewReader(payload)
	objectID, _ := r.ReadD()
	supportID, _ := r.ReadD()
	return &RequestEnchantItem{ObjectID: objectID, SupportID: supportID}
}
