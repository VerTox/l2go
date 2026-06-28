package l2pkt

import (
	"encoding/binary"
	"errors"
	"math"
	"unicode/utf16"
)

var (
	ErrUnderflow = errors.New("buffer underflow")
)

// Reader — маленький LE-ридер поверх байтового среза.
// Поддерживает readB/C/H/D/Q/F/S, hasRemaining и т.д.
type Reader struct {
	buf   []byte
	off   int
	order binary.ByteOrder
	// небольшой реюз-бак для readS (избежим лишних аллокаций)
	u16scratch []uint16
}

func NewReader(b []byte) *Reader {
	return &Reader{buf: b, order: binary.LittleEndian}
}

func (r *Reader) Reset(b []byte) { r.buf = b; r.off = 0; r.u16scratch = r.u16scratch[:0] }

func (r *Reader) HasRemaining() bool { return r.off < len(r.buf) }
func (r *Reader) Remaining() int     { return len(r.buf) - r.off }
func (r *Reader) Offset() int        { return r.off }
func (r *Reader) Slice() []byte      { return r.buf[r.off:] }

// ---- readB ----

func (r *Reader) ReadB(dst []byte) error {
	n := len(dst)
	if r.off+n > len(r.buf) {
		return ErrUnderflow
	}
	copy(dst, r.buf[r.off:r.off+n])
	r.off += n
	return nil
}

func (r *Reader) ReadBInto(dst []byte, offset, n int) error {
	if r.off+n > len(r.buf) || offset < 0 || offset+n > len(dst) {
		return ErrUnderflow
	}
	copy(dst[offset:offset+n], r.buf[r.off:r.off+n])
	r.off += n
	return nil
}

// ---- integers ----

func (r *Reader) ReadC() (uint8, error) {
	if r.off+1 > len(r.buf) {
		return 0, ErrUnderflow
	}
	v := r.buf[r.off]
	r.off++
	return v, nil
}

func (r *Reader) ReadH() (uint16, error) {
	if r.off+2 > len(r.buf) {
		return 0, ErrUnderflow
	}
	v := r.order.Uint16(r.buf[r.off : r.off+2])
	r.off += 2
	return v, nil
}

func (r *Reader) ReadD() (int32, error) {
	if r.off+4 > len(r.buf) {
		return 0, ErrUnderflow
	}
	v := int32(r.order.Uint32(r.buf[r.off : r.off+4]))
	r.off += 4
	return v, nil
}

func (r *Reader) ReadQ() (int64, error) {
	if r.off+8 > len(r.buf) {
		return 0, ErrUnderflow
	}
	v := int64(r.order.Uint64(r.buf[r.off : r.off+8]))
	r.off += 8
	return v, nil
}

func (r *Reader) ReadF() (float64, error) {
	u, err := r.ReadQ()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(uint64(u)), nil
}

// ---- strings (UTF-16LE, zero-terminated) ----

func (r *Reader) ReadS() (string, error) {
	r.u16scratch = r.u16scratch[:0]
	for {
		// берём 2 байта как uint16 (LE)
		if r.off+2 > len(r.buf) {
			return "", ErrUnderflow
		}
		u := r.order.Uint16(r.buf[r.off : r.off+2])
		r.off += 2
		if u == 0 { // терминатор
			break
		}
		r.u16scratch = append(r.u16scratch, u)
	}
	// конвертация UTF-16 → string
	if len(r.u16scratch) == 0 {
		return "", nil
	}
	runes := utf16.Decode(r.u16scratch)
	return string(runes), nil
}
