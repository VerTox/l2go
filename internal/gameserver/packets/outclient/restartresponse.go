package outclient

import (
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// RestartResponse packet (0x71) - response to restart request
// Based on Java L2J: RestartResponse.java
// Format: cd (opcode + success flag)
type RestartResponse struct {
	Success bool // Whether restart was successful
}

// NewRestartResponse creates a new RestartResponse packet
func NewRestartResponse(success bool) *RestartResponse {
	return &RestartResponse{
		Success: success,
	}
}

// Build creates the binary packet data
func (p *RestartResponse) Build() []byte {
	writer := l2pkt.NewWriter()
	
	// Write packet opcode (corrected to match Java L2J)
	writer.WriteC(0x71)
	
	// Write success flag
	if p.Success {
		writer.WriteD(1)
	} else {
		writer.WriteD(0)
	}
	
	return writer.Bytes()
}

// GetPacketData returns packet data for l2pkt compatibility
func (p *RestartResponse) GetPacketData() []byte {
	return p.Build()
}

// GetData returns packet data (alternative interface)
func (p *RestartResponse) GetData() []byte {
	return p.Build()
}