package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// RequestAutoSoulShot packet (0xd0:0x0d) - configure auto-shot settings
type RequestAutoSoulShot struct {
	ItemID   int32 `json:"item_id"`
	Activate bool  `json:"activate"`
}

func (p *RequestAutoSoulShot) Read(r *l2pkt.Reader) bool {
	itemID, err := r.ReadD()
	if err != nil {
		return false
	}
	p.ItemID = itemID
	
	activate, err := r.ReadD()
	if err != nil {
		return false
	}
	p.Activate = activate != 0
	
	return true
}