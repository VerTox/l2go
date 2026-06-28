package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// KeyPacket mirrors L2J: opcode 0x2e, id(1=ok), first 8 bytes of key, flags and server id.
func NewKeyPacket(key16 []byte, protocolOK bool, serverID int, obfuscationKey uint32) []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x2e)
	if protocolOK {
		b.WriteC(0x01)
	} else {
		b.WriteC(0x00)
	}
	// write first 8 bytes of dynamic key
	for i := 0; i < 8; i++ {
		b.WriteC(key16[i])
	}
	b.WriteD(0x01) // unknown/flag
	b.WriteD(int32(serverID))
	b.WriteC(0x01)                  // unknown/flag
	b.WriteD(int32(obfuscationKey)) // opcode obfuscation key (0 disables)
	return b.Bytes()
}
