package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// RequestKeyMapping packet (0xd0:0x21) - request current key mappings
type RequestKeyMapping struct {
	// No data needed - just a request
}

func (p *RequestKeyMapping) Read(r *l2pkt.Reader) bool {
	// No data to read for this packet
	return true
}

// RequestSaveKeyMapping packet (0xd0:0x22) - save key mappings
type RequestSaveKeyMapping struct {
	// Key mapping data would be complex, implement when needed
	Data []byte `json:"data"`
}

func (p *RequestSaveKeyMapping) Read(r *l2pkt.Reader) bool {
	// Read remaining data as raw bytes for now
	p.Data = r.Slice()
	return true
}