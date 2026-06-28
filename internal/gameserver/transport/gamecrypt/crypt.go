package gamecrypt

// Crypt implements L2J GameCrypt (16-byte key, direction-specific state).
// Algorithm mirrors l2jserver GameCrypt.java.
type Crypt struct {
    inKey   [16]byte
    outKey  [16]byte
    enabled bool
}

func New() *Crypt { return &Crypt{} }

// SetKey sets both directions' keys to the provided 16-byte key.
func (c *Crypt) SetKey(key []byte) {
    if len(key) >= 16 {
        copy(c.inKey[:], key[:16])
        copy(c.outKey[:], key[:16])
    }
}

// Decrypt in-place (no-op if not enabled).
func (c *Crypt) Decrypt(raw []byte) {
    if !c.enabled {
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
    if !c.enabled {
        c.enabled = true
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

