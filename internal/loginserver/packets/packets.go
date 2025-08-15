package packets

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
)

type Buffer struct {
	bytes.Buffer
}

func NewBuffer() *Buffer {
	return &Buffer{}
}

func (b *Buffer) WriteUInt64(value uint64) {
	binary.Write(b, binary.LittleEndian, value)
}

func (b *Buffer) WriteUInt32(value uint32) {
	binary.Write(b, binary.LittleEndian, value)
}

func (b *Buffer) WriteUInt16(value uint16) {
	binary.Write(b, binary.LittleEndian, value)
}

func (b *Buffer) WriteUInt8(value uint8) {
	binary.Write(b, binary.LittleEndian, value)
}

func (b *Buffer) WriteFloat64(value float64) {
	binary.Write(b, binary.LittleEndian, value)
}

func (b *Buffer) WriteFloat32(value float32) {
	binary.Write(b, binary.LittleEndian, value)
}

// WriteString writes a UTF-16LE encoded string with null terminator (like Java writeS)
func (b *Buffer) WriteString(str string) {
	// Convert string to UTF-16
	utf16Runes := utf16.Encode([]rune(str))

	// Write each UTF-16 code unit as little-endian uint16
	for _, r := range utf16Runes {
		b.WriteUInt16(r)
	}

	// Add null terminator (2 bytes of zero)
	b.WriteUInt16(0)
}

type Reader struct {
	*bytes.Reader
}

func NewReader(buffer []byte) *Reader {
	return &Reader{bytes.NewReader(buffer)}
}

func (r *Reader) ReadBytes(number int) []byte {
	buffer := make([]byte, number)
	n, _ := r.Read(buffer)
	if n < number {
		return []byte{}
	}

	return buffer
}

func (r *Reader) ReadUInt64() uint64 {
	var result uint64

	buffer := make([]byte, 8)
	n, _ := r.Read(buffer)
	if n < 8 {
		return 0
	}

	buf := bytes.NewBuffer(buffer)

	binary.Read(buf, binary.LittleEndian, &result)

	return result
}

func (r *Reader) ReadUInt32() uint32 {
	var result uint32

	buffer := make([]byte, 4)
	n, _ := r.Read(buffer)
	if n < 4 {
		return 0
	}

	buf := bytes.NewBuffer(buffer)

	binary.Read(buf, binary.LittleEndian, &result)

	return result
}

func (r *Reader) ReadUInt16() uint16 {
	var result uint16

	buffer := make([]byte, 2)
	n, _ := r.Read(buffer)
	if n < 2 {
		return 0
	}

	buf := bytes.NewBuffer(buffer)

	binary.Read(buf, binary.LittleEndian, &result)

	return result
}

func (r *Reader) ReadUInt8() uint8 {
	var result uint8

	buffer := make([]byte, 1)
	n, _ := r.Read(buffer)
	if n < 1 {
		return 0
	}

	buf := bytes.NewBuffer(buffer)

	binary.Read(buf, binary.LittleEndian, &result)

	return result
}

func (r *Reader) ReadString() string {
	var utf16Codes []uint16

	for {
		// Read a UTF-16 code unit (2 bytes, little-endian)
		codeUnit := r.ReadUInt16()
		if codeUnit == 0 {
			break // Null terminator
		}
		utf16Codes = append(utf16Codes, codeUnit)
	}

	// Decode UTF-16 to Go string (UTF-8)
	runes := utf16.Decode(utf16Codes)
	return string(runes)
}
