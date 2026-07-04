package stressclient

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	lspackets "github.com/VerTox/l2go/internal/loginserver/packets"
)

// TestUnscrambleModulusInvertsServerScramble verifies our client-side unscramble
// is the exact inverse of the server's ScrambleModulus, so the RSA modulus we
// recover from Init matches the server's real public key.
func TestUnscrambleModulusInvertsServerScramble(t *testing.T) {
	orig := make([]byte, 128)
	for i := range orig {
		orig[i] = byte((i*31 + 7) & 0xFF)
	}
	scrambled := make([]byte, 128)
	copy(scrambled, orig)

	// Server scrambles the modulus before putting it on the wire.
	lspackets.ScrambleModulus(scrambled)
	if bytes.Equal(scrambled, orig) {
		t.Fatal("scramble was a no-op; test setup wrong")
	}

	// Client unscrambles what it reads from Init.
	got := unscrambleModulus(scrambled)
	if !bytes.Equal(got, orig) {
		t.Errorf("unscramble(scramble(x)) != x\n orig=%x\n got =%x", orig, got)
	}
}

// TestRsaEncryptNoPadRoundtrips verifies the credential block we encrypt with the
// server's public modulus decrypts back to the same block with user/pass at the
// offsets the server reads (0x5E / 0x6C).
func TestRsaEncryptNoPadRoundtrips(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}

	const user, pass = "stress0042", "stress0042"
	block := make([]byte, 128)
	copy(block[0x5E:], []byte(user))
	copy(block[0x6C:], []byte(pass))

	cipher := rsaEncryptNoPad(key.PublicKey.N.Bytes(), block)
	if len(cipher) != 128 {
		t.Fatalf("cipher len = %d, want 128", len(cipher))
	}

	// Decrypt exactly like the server (RsaDecryptNoPadding: c^d mod n).
	dec, err := lspackets.RsaDecryptNoPadding(key, cipher)
	if err != nil {
		t.Fatal(err)
	}
	gotUser := string(bytes.Trim(dec[0x5E:0x5E+14], "\x00"))
	gotPass := string(bytes.Trim(dec[0x6C:0x6C+16], "\x00"))
	if gotUser != user {
		t.Errorf("decrypted user = %q, want %q", gotUser, user)
	}
	if gotPass != pass {
		t.Errorf("decrypted pass = %q, want %q", gotPass, pass)
	}
}
