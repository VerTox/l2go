package stressclient

import (
	"bytes"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/transport/gamecrypt"
)

// TestGameCryptStreamSync mirrors the exact handshake enable/prime sequence the
// client uses against the server and verifies both XOR streams stay in sync
// across several packets in both directions. This is the subtle part of the GS
// handshake (the first Encrypt only enables; the client primes to match), so it
// gets an isolated regression test.
func TestGameCryptStreamSync(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 0xc8, 0x27, 0x93, 0x01, 0xa1, 0x6c, 0x31, 0x97}

	server := gamecrypt.New()
	server.SetKey(key)
	client := gamecrypt.New()
	client.SetKey(key)

	// Server enables its crypt by "sending" the KeyPacket (first Encrypt is a
	// no-op that just flips enabled). The client received that raw, then primes.
	server.Encrypt(nil)
	client.Encrypt(nil)

	// A few server->client packets of differing lengths (key advances by size).
	for _, msg := range [][]byte{
		[]byte("UserInfo payload here"),
		[]byte("ItemList"),
		[]byte("SkillList and a longer body to vary the length....."),
	} {
		enc := append([]byte(nil), msg...)
		server.Encrypt(enc)
		if bytes.Equal(enc, msg) {
			t.Fatalf("server->client packet was not encrypted: %q", msg)
		}
		client.Decrypt(enc)
		if !bytes.Equal(enc, msg) {
			t.Fatalf("server->client desync: got %q want %q", enc, msg)
		}
	}

	// And client->server packets.
	for _, msg := range [][]byte{
		[]byte("AuthLogin"),
		[]byte("CharacterSelect slot 0"),
		[]byte("EnterWorld"),
	} {
		enc := append([]byte(nil), msg...)
		client.Encrypt(enc)
		if bytes.Equal(enc, msg) {
			t.Fatalf("client->server packet was not encrypted: %q", msg)
		}
		server.Decrypt(enc)
		if !bytes.Equal(enc, msg) {
			t.Fatalf("client->server desync: got %q want %q", enc, msg)
		}
	}
}
