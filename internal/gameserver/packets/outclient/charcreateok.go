package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// CharCreateOk (opcode 0x0f) - successful character creation response.
func NewCharCreateOk(success bool) []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x0f)
	if success {
		b.WriteC(0x01)
	} else {
		b.WriteC(0x00)
	}
	return b.Bytes()
}
