package blowfish

import (
	"bytes"
	"testing"
)

// NewCipher must produce a cipher whose Encrypt/Decrypt round-trips. This also
// exercises ExpandKey (via NewCipher) and decryptBlock (via Decrypt), which the
// salted-cipher tests don't reach. Exact ciphertext values aren't asserted: this
// L2J variant uses little-endian block I/O, so the standard KAT vectors (which
// are big-endian) don't apply — but a round-trip is byte-order agnostic.
func TestNewCipherRoundTrip(t *testing.T) {
	for i, tt := range encryptTests {
		c, err := NewCipher(tt.key)
		if err != nil {
			t.Fatalf("case %d: NewCipher: %v", i, err)
		}

		enc := make([]byte, BlockSize)
		c.Encrypt(enc, tt.in)

		dec := make([]byte, BlockSize)
		c.Decrypt(dec, enc)
		if !bytes.Equal(dec, tt.in) {
			t.Errorf("case %d: round-trip mismatch: got %x, want %x", i, dec, tt.in)
		}
	}
}

func TestNewCipherKeySize(t *testing.T) {
	// Too short (empty).
	if _, err := NewCipher(nil); err != KeySizeError(0) {
		t.Errorf("NewCipher(nil): got %#v, want %#v", err, KeySizeError(0))
	}
	// Too long (> 56 bytes).
	long := make([]byte, 57)
	if _, err := NewCipher(long); err != KeySizeError(57) {
		t.Errorf("NewCipher(57 bytes): got %#v, want %#v", err, KeySizeError(57))
	}
	// The error message renders the offending size.
	if got := KeySizeError(57).Error(); got != "crypto/blowfish: invalid key size 57" {
		t.Errorf("KeySizeError.Error(): got %q", got)
	}
}

// An empty salt makes NewSaltedCipher delegate to NewCipher.
func TestNewSaltedCipherEmptySalt(t *testing.T) {
	c, err := NewSaltedCipher([]byte("a key"), nil)
	if err != nil {
		t.Fatalf("NewSaltedCipher(key, nil): %v", err)
	}
	if c == nil {
		t.Fatal("NewSaltedCipher(key, nil): got nil cipher")
	}
}

func TestBlockSize(t *testing.T) {
	c, err := NewCipher([]byte("a key"))
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	if got := c.BlockSize(); got != 8 { // BlockSize const is 8
		t.Errorf("BlockSize() = %d, want 8", got)
	}
}
