package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// chatTypeTell is the TELL channel id — the only type that carries a trailing
// target-name string on the wire (L2J Say2: _target = (type == TELL) ? readS()).
const chatTypeTell int32 = 2

// Say2 is the client chat packet (opcode 0x49): a chat line for some channel.
type Say2 struct {
	Text   string
	Type   int32
	Target string // populated only for TELL
}

// NewSay2 parses a Say2 packet from payload: readS(text), readD(type), and for
// TELL an extra readS(target). Matches L2J Say2.readImpl.
func NewSay2(payload []byte) *Say2 {
	r := l2pkt.NewReader(payload)
	text, _ := r.ReadS()
	typ, _ := r.ReadD()
	s := &Say2{Text: text, Type: typ}
	if typ == chatTypeTell {
		target, _ := r.ReadS()
		s.Target = target
	}
	return s
}
