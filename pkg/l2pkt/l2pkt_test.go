package l2pkt

import (
	"errors"
	"testing"
)

// A Writer output must read back identically through a Reader.
func TestWriterReaderRoundTrip(t *testing.T) {
	w := NewWriter()
	w.WriteC(0x12)
	w.WriteH(0x3456)
	w.WriteD(-123456)
	w.WriteQ(-1234567890123)
	w.WriteF(3.14159)
	w.WriteS("Héllo, 世界")
	w.WriteB([]byte{0xDE, 0xAD, 0xBE, 0xEF})

	r := NewReader(w.Bytes())

	if c, err := r.ReadC(); err != nil || c != 0x12 {
		t.Fatalf("ReadC = %#x, %v", c, err)
	}
	if h, err := r.ReadH(); err != nil || h != 0x3456 {
		t.Fatalf("ReadH = %#x, %v", h, err)
	}
	if d, err := r.ReadD(); err != nil || d != -123456 {
		t.Fatalf("ReadD = %d, %v", d, err)
	}
	if q, err := r.ReadQ(); err != nil || q != -1234567890123 {
		t.Fatalf("ReadQ = %d, %v", q, err)
	}
	if f, err := r.ReadF(); err != nil || f != 3.14159 {
		t.Fatalf("ReadF = %v, %v", f, err)
	}
	if s, err := r.ReadS(); err != nil || s != "Héllo, 世界" {
		t.Fatalf("ReadS = %q, %v", s, err)
	}
	b := make([]byte, 4)
	if err := r.ReadB(b); err != nil {
		t.Fatalf("ReadB: %v", err)
	}
	if b[0] != 0xDE || b[1] != 0xAD || b[2] != 0xBE || b[3] != 0xEF {
		t.Fatalf("ReadB = %x", b)
	}
	if r.HasRemaining() {
		t.Errorf("buffer not fully consumed: %d bytes remain", r.Remaining())
	}
}

// Offset/Remaining/Slice/Reset track the cursor correctly.
func TestReaderNavigation(t *testing.T) {
	r := NewReader([]byte{1, 2, 3, 4, 5})
	if r.Offset() != 0 || r.Remaining() != 5 || !r.HasRemaining() {
		t.Fatalf("initial: off=%d rem=%d", r.Offset(), r.Remaining())
	}
	_, _ = r.ReadC()
	_, _ = r.ReadH()
	if r.Offset() != 3 || r.Remaining() != 2 {
		t.Fatalf("after 3 bytes: off=%d rem=%d", r.Offset(), r.Remaining())
	}
	if got := r.Slice(); len(got) != 2 || got[0] != 4 || got[1] != 5 {
		t.Fatalf("Slice = %v", got)
	}
	r.Reset([]byte{9})
	if r.Offset() != 0 || r.Remaining() != 1 {
		t.Fatalf("Reset did not rewind: off=%d rem=%d", r.Offset(), r.Remaining())
	}
}

// Every typed read returns ErrUnderflow on an exhausted buffer.
func TestReaderUnderflow(t *testing.T) {
	empty := func() *Reader { return NewReader(nil) }
	if _, err := empty().ReadC(); !errors.Is(err, ErrUnderflow) {
		t.Error("ReadC")
	}
	if _, err := empty().ReadH(); !errors.Is(err, ErrUnderflow) {
		t.Error("ReadH")
	}
	if _, err := empty().ReadD(); !errors.Is(err, ErrUnderflow) {
		t.Error("ReadD")
	}
	if _, err := empty().ReadQ(); !errors.Is(err, ErrUnderflow) {
		t.Error("ReadQ")
	}
	if _, err := empty().ReadF(); !errors.Is(err, ErrUnderflow) {
		t.Error("ReadF")
	}
	if err := empty().ReadB(make([]byte, 1)); !errors.Is(err, ErrUnderflow) {
		t.Error("ReadB")
	}
	if _, err := empty().ReadS(); !errors.Is(err, ErrUnderflow) {
		t.Error("ReadS")
	}
}

func TestReadBInto(t *testing.T) {
	r := NewReader([]byte{0xAA, 0xBB, 0xCC})
	dst := make([]byte, 5)
	if err := r.ReadBInto(dst, 1, 3); err != nil {
		t.Fatalf("ReadBInto: %v", err)
	}
	if dst[0] != 0 || dst[1] != 0xAA || dst[2] != 0xBB || dst[3] != 0xCC || dst[4] != 0 {
		t.Fatalf("dst = %x", dst)
	}

	// Negative offset into dst.
	if err := NewReader([]byte{1, 2}).ReadBInto(make([]byte, 2), -1, 1); !errors.Is(err, ErrUnderflow) {
		t.Error("expected underflow for negative offset")
	}
	// Writing past the end of dst.
	if err := NewReader([]byte{1, 2}).ReadBInto(make([]byte, 2), 1, 2); !errors.Is(err, ErrUnderflow) {
		t.Error("expected underflow writing past dst")
	}
	// Reading past the end of the source buffer.
	if err := NewReader([]byte{1}).ReadBInto(make([]byte, 4), 0, 2); !errors.Is(err, ErrUnderflow) {
		t.Error("expected underflow reading past src")
	}
}

func TestReadStringCases(t *testing.T) {
	// Empty string = just the 16-bit null terminator.
	if s, err := NewReader([]byte{0x00, 0x00}).ReadS(); err != nil || s != "" {
		t.Fatalf("empty: got %q, %v", s, err)
	}
	// "A" followed by the terminator.
	if s, err := NewReader([]byte{0x41, 0x00, 0x00, 0x00}).ReadS(); err != nil || s != "A" {
		t.Fatalf(`"A": got %q, %v`, s, err)
	}
	// Non-terminated string runs off the end → underflow.
	if _, err := NewReader([]byte{0x41, 0x00}).ReadS(); !errors.Is(err, ErrUnderflow) {
		t.Error("expected underflow for unterminated string")
	}
}

func TestWriterAliasesAndReset(t *testing.T) {
	w := NewWriter()
	w.PutInt(1)      // WriteD alias, 4 bytes
	w.PutDouble(2.0) // WriteF alias, 8 bytes
	w.PutFloat(1.5)  // 32-bit float, 4 bytes
	if w.Len() != 16 {
		t.Fatalf("Len = %d, want 16", w.Len())
	}
	w.Reset()
	if w.Len() != 0 {
		t.Fatalf("after Reset Len = %d, want 0", w.Len())
	}
}

// echoPacket exercises the SendablePacket/ReceivablePacket round trip.
type echoPacket struct {
	c byte
	d int32
}

func (p *echoPacket) Write(w *Writer) {
	w.WriteC(p.c)
	w.WriteD(p.d)
}

func (p *echoPacket) Read(r *Reader) bool {
	c, err := r.ReadC()
	if err != nil {
		return false
	}
	d, err := r.ReadD()
	if err != nil {
		return false
	}
	p.c, p.d = c, d
	return true
}

func TestBuildAndParsePacket(t *testing.T) {
	payload := BuildPacket(&echoPacket{c: 0x7F, d: -42})
	if len(payload) != 5 {
		t.Fatalf("payload len = %d, want 5", len(payload))
	}

	var out echoPacket
	if !ParsePacket(payload, &out) {
		t.Fatal("ParsePacket returned false for a well-formed payload")
	}
	if out.c != 0x7F || out.d != -42 {
		t.Fatalf("parsed = %#x, %d", out.c, out.d)
	}

	// Trailing bytes → not fully consumed → false.
	if ParsePacket(append(payload, 0x00), &echoPacket{}) {
		t.Error("ParsePacket should reject payloads with trailing bytes")
	}
	// Truncated payload → Read fails → false.
	if ParsePacket(payload[:3], &echoPacket{}) {
		t.Error("ParsePacket should reject truncated payloads")
	}
}
