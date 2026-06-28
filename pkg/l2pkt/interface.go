package l2pkt

type ReceivablePacket interface {
	Read(r *Reader) bool
}

type SendablePacket interface {
	Write(w *Writer)
}

func BuildPacket(p SendablePacket) []byte {
	w := NewWriter()
	p.Write(w)
	return w.Bytes()
}

func ParsePacket(payload []byte, p ReceivablePacket) bool {
	r := NewReader(payload)
	if !p.Read(r) {
		return false
	}
	if !r.HasRemaining() {
		return true
	}
	return false
}
