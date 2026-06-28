package l2pkt

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
)

// Writer — аналог Java ByteBuffer с методами writeC/H/D/Q/F/B/S.
// По умолчанию работает в Little Endian (как L2J).
type Writer struct {
	buf   bytes.Buffer
	order binary.ByteOrder
}

func NewWriter() *Writer {
	w := &Writer{order: binary.LittleEndian}
	return w
}

func (w *Writer) Bytes() []byte   { return w.buf.Bytes() }
func (w *Writer) Len() int        { return w.buf.Len() }
func (w *Writer) Reset()          { w.buf.Reset() }
func (w *Writer) WriteB(b []byte) { _, _ = w.buf.Write(b) } // writeB(byte[])
func (w *Writer) write(v any)     { _ = binary.Write(&w.buf, w.order, v) }

// ——— put*/write* эквиваленты ———

// putInt / writeD: 32-бит int
func (w *Writer) PutInt(v int32) { w.write(v) }
func (w *Writer) WriteD(v int32) { w.write(v) }

// putDouble / writeF: double (64-бит)
func (w *Writer) PutDouble(v float64) { w.write(v) }
func (w *Writer) WriteF(v float64)    { w.write(v) }

// putFloat: 32-бит float
func (w *Writer) PutFloat(v float32) { w.write(v) }

// writeC: 8-бит
func (w *Writer) WriteC(v byte) { _ = w.buf.WriteByte(v) }

// writeH: 16-бит
func (w *Writer) WriteH(v uint16) { w.write(v) }

// writeQ: 64-бит
func (w *Writer) WriteQ(v int64) { w.write(v) }

// writeS: UTF-16LE нуль-терминированная строка (как ByteBuffer.putChar + '\0')
func (w *Writer) WriteS(s string) {
	if s != "" {
		u16 := utf16.Encode([]rune(s))
		for _, cu := range u16 {
			w.write(cu) // cu пишется как uint16 в LE
		}
	}
	// нулевой 16-битный терминатор
	w.write(uint16(0))
}
