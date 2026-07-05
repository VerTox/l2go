package crypt

import (
	"bytes"
	"testing"
)

// Blowfish wrappers must round-trip: encrypting then decrypting with the same
// key restores the original plaintext (ECB over independent 8-byte blocks).
func TestBlowfishRoundTrip(t *testing.T) {
	key := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF}
	plain := []byte("L2Go-16-bytes!!!") // exactly 16 bytes, two blocks

	enc, err := BlowfishEncrypt(plain, key)
	if err != nil {
		t.Fatalf("BlowfishEncrypt: %v", err)
	}
	if bytes.Equal(enc, plain) {
		t.Fatal("ciphertext equals plaintext; encryption did nothing")
	}

	dec, err := BlowfishDecrypt(enc, key)
	if err != nil {
		t.Fatalf("BlowfishDecrypt: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatalf("round-trip mismatch: got %x, want %x", dec, plain)
	}
}

func TestBlowfishErrors(t *testing.T) {
	key := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF}

	// Data length not a multiple of the 8-byte block size.
	if _, err := BlowfishEncrypt(make([]byte, 7), key); err == nil {
		t.Error("BlowfishEncrypt: expected error for non-block-aligned data")
	}
	if _, err := BlowfishDecrypt(make([]byte, 7), key); err == nil {
		t.Error("BlowfishDecrypt: expected error for non-block-aligned data")
	}

	// Invalid (empty) key rejected by the underlying cipher.
	if _, err := BlowfishEncrypt(make([]byte, 8), nil); err == nil {
		t.Error("BlowfishEncrypt: expected error for empty key")
	}
	if _, err := BlowfishDecrypt(make([]byte, 8), nil); err == nil {
		t.Error("BlowfishDecrypt: expected error for empty key")
	}
}

// The static (Init) and GameServer helpers must round-trip against their fixed keys.
func TestBlowfishStaticAndGameServerRoundTrip(t *testing.T) {
	plain := []byte("eight999sixteen!") // 16 bytes

	enc, err := BlowfishEncryptStatic(plain)
	if err != nil {
		t.Fatalf("BlowfishEncryptStatic: %v", err)
	}
	dec, err := BlowfishDecrypt(enc, StaticBlowfishKey)
	if err != nil {
		t.Fatalf("BlowfishDecrypt(static): %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Errorf("static round-trip mismatch: got %x, want %x", dec, plain)
	}

	enc, err = BlowfishEncryptGameServer(plain)
	if err != nil {
		t.Fatalf("BlowfishEncryptGameServer: %v", err)
	}
	dec, err = BlowfishDecryptGameServer(enc)
	if err != nil {
		t.Fatalf("BlowfishDecryptGameServer: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Errorf("gameserver round-trip mismatch: got %x, want %x", dec, plain)
	}
}

// Checksum is a thin alias for VerifyChecksum.
func TestChecksumDelegatesToVerify(t *testing.T) {
	valid := []byte{0x01, 0x02, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}
	AppendChecksum(valid)
	if !Checksum(valid) {
		t.Error("Checksum should accept a packet with a valid checksum")
	}

	invalid := []byte{0x01, 0x02, 0x03, 0x04, 0xFF, 0xFF, 0xFF, 0xFF}
	if Checksum(invalid) {
		t.Error("Checksum should reject a packet with a bad checksum")
	}
}

// AppendChecksum is a no-op for a buffer that isn't a positive multiple of 4.
func TestAppendChecksumInvalidSize(t *testing.T) {
	raw := []byte{1, 2, 3} // size 3: too small and not 4-aligned
	cp := append([]byte(nil), raw...)
	AppendChecksum(raw)
	if !bytes.Equal(raw, cp) {
		t.Errorf("AppendChecksum mutated an invalid-size buffer: got %x, want %x", raw, cp)
	}
}

// EncXORPass followed by DecXORPass must restore the data region and clear the
// key slot. Buffer layout: [4-byte header][data blocks][4-byte key][4-byte tail].
func TestXORPassRoundTrip(t *testing.T) {
	const size = 24 // header(4) + data(12) + key(4) + tail(4)
	raw := make([]byte, size)
	header := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	copy(raw[0:4], header)
	data := []byte{
		0x10, 0x20, 0x30, 0x40,
		0x50, 0x60, 0x70, 0x80,
		0x90, 0xA0, 0xB0, 0xC0,
	}
	copy(raw[4:16], data)

	EncXORPass(raw, 0, size, 0x12345678)
	if bytes.Equal(raw[4:16], data) {
		t.Fatal("EncXORPass left the data region unchanged")
	}

	DecXORPass(raw, 0, size)

	if !bytes.Equal(raw[4:16], data) {
		t.Errorf("data region not restored: got %x, want %x", raw[4:16], data)
	}
	if !bytes.Equal(raw[0:4], header) {
		t.Errorf("header must not change: got %x, want %x", raw[0:4], header)
	}
	for i, b := range raw[16:20] {
		if b != 0 {
			t.Errorf("key slot byte %d not cleared: %#x", i, b)
		}
	}
}

// DecXORPass is a no-op when there is not enough data for a key slot.
func TestDecXORPassTooShort(t *testing.T) {
	raw := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	cp := append([]byte(nil), raw...)
	DecXORPass(raw, 0, len(raw)) // size == 8 → early return
	if !bytes.Equal(raw, cp) {
		t.Errorf("DecXORPass mutated a too-short buffer: got %x, want %x", raw, cp)
	}
}
