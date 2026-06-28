package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// LeaveWorld packet (0x84) - signals client to exit gracefully
// This packet tells the client to perform a clean logout without showing "Connection lost" dialog
// Based on Java L2J Server implementation: client.close(LeaveWorld.STATIC_PACKET)
type LeaveWorld struct{}

// NewLeaveWorld creates a new LeaveWorld packet
func NewLeaveWorld() *LeaveWorld {
	return &LeaveWorld{}
}

// GetData returns the packet data for LeaveWorld (0x84)
// This is a simple packet with just the opcode
func (p *LeaveWorld) GetData() []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x84)
	return w.Bytes()
}