package gamecrypt

// Crypt implements L2J GameCrypt (16-byte key, direction-specific state).
// Algorithm mirrors l2jserver GameCrypt.java.
//
// The enable flag is split per direction (inEnabled/outEnabled) so the two
// directions can be driven by different goroutines without sharing a field: with
// the async send path (l2go-e9q) Encrypt runs on a per-connection writer goroutine
// while Decrypt runs on the read loop. inEnabled is flipped explicitly via
// EnableDecrypt on the read-loop goroutine; outEnabled self-flips on the first
// Encrypt (first packet in clear, L2J behavior) on the writer goroutine. inKey is
// only ever touched by Decrypt, outKey only by Encrypt, so no direction shares
// mutable state across goroutines.
type Crypt struct {
    inKey      [16]byte
    outKey     [16]byte
    inEnabled  bool
    outEnabled bool
}

func New() *Crypt { return &Crypt{} }

// SetKey sets both directions' keys to the provided 16-byte key.
func (c *Crypt) SetKey(key []byte) {
    if len(key) >= 16 {
        copy(c.inKey[:], key[:16])
        copy(c.outKey[:], key[:16])
    }
}

// EnableDecrypt turns on inbound decryption. Call once, after SetKey, on the same
// goroutine that calls Decrypt. Kept separate from the outbound enable (which the
// first Encrypt flips) so the read loop and the send-writer goroutine never share
// an enable flag. (l2go-e9q)
func (c *Crypt) EnableDecrypt() { c.inEnabled = true }

// Decrypt in-place (no-op if not enabled).
func (c *Crypt) Decrypt(raw []byte) {
    if !c.inEnabled {
        return
    }
    temp := 0
    for i := 0; i < len(raw); i++ {
        temp2 := int(raw[i]) & 0xFF
        raw[i] = byte(temp2 ^ int(c.inKey[i&15]) ^ temp)
        temp = temp2
    }
    // advance bytes 8..11 by size
    old := int(c.inKey[8]) & 0xFF
    old |= (int(c.inKey[9]) << 8) & 0xFF00
    old |= (int(c.inKey[10]) << 16) & 0xFF0000
    old |= (int(c.inKey[11]) << 24) & 0xFF000000
    old += len(raw)
    c.inKey[8] = byte(old & 0xFF)
    c.inKey[9] = byte((old >> 8) & 0xFF)
    c.inKey[10] = byte((old >> 16) & 0xFF)
    c.inKey[11] = byte((old >> 24) & 0xFF)
}

// Encrypt in-place; first call only enables and returns without encrypting (matches L2J behavior).
func (c *Crypt) Encrypt(raw []byte) {
    if !c.outEnabled {
        c.outEnabled = true
        return
    }
    temp := 0
    for i := 0; i < len(raw); i++ {
        temp2 := int(raw[i]) & 0xFF
        temp = temp2 ^ int(c.outKey[i&15]) ^ temp
        raw[i] = byte(temp)
    }
    // advance bytes 8..11 by size
    old := int(c.outKey[8]) & 0xFF
    old |= (int(c.outKey[9]) << 8) & 0xFF00
    old |= (int(c.outKey[10]) << 16) & 0xFF0000
    old |= (int(c.outKey[11]) << 24) & 0xFF000000
    old += len(raw)
    c.outKey[8] = byte(old & 0xFF)
    c.outKey[9] = byte((old >> 8) & 0xFF)
    c.outKey[10] = byte((old >> 16) & 0xFF)
    c.outKey[11] = byte((old >> 24) & 0xFF)
}

