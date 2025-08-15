package crypt

import (
	"errors"

	"github.com/VerTox/l2go/pkg/crypt/blowfish"
)

// Static Blowfish key used for encrypting the first packet (Init) to clients
// This is the same key as used in L2J Server
var StaticBlowfishKey = []byte{
	0x6b, 0x60, 0xcb, 0x5b, 0x82, 0xce, 0x90, 0xb1,
	0xcc, 0x2b, 0x6c, 0x55, 0x6c, 0x6c, 0x6c, 0x6c,
}

// Static GameServer Blowfish key for LoginServer ↔ GameServer communication
// Matches Java: "_;v.]05-31!|+-%xT!^[$\00" (22 bytes)
var GameServerBlowfishKey = []byte{
	0x5f, 0x3b, 0x76, 0x2e, 0x5d, 0x30, 0x35, 0x2d, // _;v.]05-
	0x33, 0x31, 0x21, 0x7c, 0x2b, 0x2d, 0x25, 0x78, // 31!|+-%x
	0x54, 0x21, 0x5e, 0x5b, 0x24, 0x00, // T!^[$\00
}

// VerifyChecksum verifies the checksum of a received packet (matches Java verifyChecksum exactly)
func VerifyChecksum(raw []byte) bool {
	offset := 0 // Starting from beginning like Java version
	size := len(raw)

	// Check if size is multiple of 4 and if there is more than only the checksum
	if ((size & 3) != 0) || (size <= 4) {
		return false
	}

	var chksum uint32 = 0
	count := offset + size - 4 // Same calculation as Java: i < count
	var check uint32
	var i int

	// Process data in 4-byte chunks (excluding last 4 bytes which are checksum)
	for i = offset; i < count; i += 4 {
		check = uint32(raw[i]) & 0xff
		check |= (uint32(raw[i+1]) << 8) & 0xff00
		check |= (uint32(raw[i+2]) << 16) & 0xff0000
		check |= (uint32(raw[i+3]) << 24) & 0xff000000

		chksum ^= check
	}

	// Read existing checksum from last 4 bytes (exactly as in Java)
	check = uint32(raw[i]) & 0xff
	check |= (uint32(raw[i+1]) << 8) & 0xff00
	check |= (uint32(raw[i+2]) << 16) & 0xff0000
	check |= (uint32(raw[i+3]) << 24) & 0xff000000

	return check == chksum
}

// VerifyChecksumClient verifies the checksum of a inclient packet
// Client packets have structure: [data][checksum 4 bytes][padding 12 bytes]
// Unlike server packets where checksum is at the end
func VerifyChecksumClient(raw []byte) bool {
	offset := 0
	size := len(raw)

	// Check if size is multiple of 4 and if there is at least checksum + padding
	if ((size & 3) != 0) || (size < 16) { // Client packets need at least 16 bytes (checksum + padding)
		return false
	}

	var chksum uint32 = 0
	count := offset + size - 16 // Exclude last 16 bytes (4 for checksum + 12 for padding)
	var check uint32
	var i int

	// Process data in 4-byte chunks (excluding checksum and padding)
	for i = offset; i < count; i += 4 {
		check = uint32(raw[i]) & 0xff
		check |= (uint32(raw[i+1]) << 8) & 0xff00
		check |= (uint32(raw[i+2]) << 16) & 0xff0000
		check |= (uint32(raw[i+3]) << 24) & 0xff000000

		chksum ^= check
	}

	// Read checksum from position count (first 4 bytes after data)
	check = uint32(raw[count]) & 0xff
	check |= (uint32(raw[count+1]) << 8) & 0xff00
	check |= (uint32(raw[count+2]) << 16) & 0xff0000
	check |= (uint32(raw[count+3]) << 24) & 0xff000000

	return check == chksum
}

// AppendChecksum adds checksum to outgoing packet (matches Java appendChecksum exactly)
func AppendChecksum(raw []byte) {
	offset := 0 // Starting from beginning like Java version
	size := len(raw)
	var chksum uint32 = 0
	count := offset + size - 4 // Same calculation as Java: i < count
	var ecx uint32
	var i int

	// Process data in 4-byte chunks (excluding last 4 bytes reserved for checksum)
	for i = offset; i < count; i += 4 {
		ecx = uint32(raw[i]) & 0xff
		ecx |= (uint32(raw[i+1]) << 8) & 0xff00
		ecx |= (uint32(raw[i+2]) << 16) & 0xff0000
		ecx |= (uint32(raw[i+3]) << 24) & 0xff000000

		chksum ^= ecx
	}

	// Read the existing checksum bytes (as in Java, though they will be overwritten)
	ecx = uint32(raw[i]) & 0xff
	ecx |= (uint32(raw[i+1]) << 8) & 0xff00
	ecx |= (uint32(raw[i+2]) << 16) & 0xff0000
	ecx |= (uint32(raw[i+3]) << 24) & 0xff000000

	// Write checksum to last 4 bytes (exactly as in Java)
	raw[i] = byte(chksum & 0xff)
	raw[i+1] = byte((chksum >> 8) & 0xff)
	raw[i+2] = byte((chksum >> 16) & 0xff)
	raw[i+3] = byte((chksum >> 24) & 0xff)
}

// Legacy checksum function (deprecated, use VerifyChecksum instead)
func Checksum(raw []byte) bool {
	return VerifyChecksum(raw)
}

func BlowfishDecrypt(encrypted, key []byte) ([]byte, error) {
	cipher, err := blowfish.NewCipher(key)

	if err != nil {
		return nil, errors.New("Couldn't initialize the blowfish cipher")
	}

	// Check if the encrypted data is a multiple of our block size
	if len(encrypted)%8 != 0 {
		return nil, errors.New("The encrypted data is not a multiple of the block size")
	}

	count := len(encrypted) / 8

	decrypted := make([]byte, len(encrypted))

	for i := 0; i < count; i++ {
		cipher.Decrypt(decrypted[i*8:], encrypted[i*8:])
	}

	return decrypted, nil
}

func BlowfishEncrypt(decrypted, key []byte) ([]byte, error) {
	cipher, err := blowfish.NewCipher(key)

	if err != nil {
		return nil, errors.New("Couldn't initialize the blowfish cipher")
	}

	// Check if the decrypted data is a multiple of our block size
	if len(decrypted)%8 != 0 {
		return nil, errors.New("The decrypted data is not a multiple of the block size")
	}

	count := len(decrypted) / 8

	encrypted := make([]byte, len(decrypted))

	for i := 0; i < count; i++ {
		cipher.Encrypt(encrypted[i*8:], decrypted[i*8:])
	}

	return encrypted, nil
}

// BlowfishEncryptStatic encrypts data using the static Blowfish key
// This is used specifically for the Init packet
func BlowfishEncryptStatic(decrypted []byte) ([]byte, error) {
	return BlowfishEncrypt(decrypted, StaticBlowfishKey)
}

// BlowfishEncryptGameServer encrypts data using the GameServer static Blowfish key
// This is used for LoginServer ↔ GameServer communication
func BlowfishEncryptGameServer(decrypted []byte) ([]byte, error) {
	return BlowfishEncrypt(decrypted, GameServerBlowfishKey)
}

// BlowfishDecryptGameServer decrypts data using the GameServer static Blowfish key
// This is used for LoginServer ↔ GameServer communication
func BlowfishDecryptGameServer(encrypted []byte) ([]byte, error) {
	return BlowfishDecrypt(encrypted, GameServerBlowfishKey)
}

// EncXORPass applies XOR pass encryption (matches Java encXORPass)
func EncXORPass(raw []byte, offset int, size int, key uint32) {
	stop := offset + size - 8
	pos := 4 + offset
	var edx uint32
	ecx := key // Initial xor key

	for pos < stop {
		// Read 4 bytes in little-endian format
		edx = uint32(raw[pos]) & 0xFF
		edx |= (uint32(raw[pos+1]) & 0xFF) << 8
		edx |= (uint32(raw[pos+2]) & 0xFF) << 16
		edx |= (uint32(raw[pos+3]) & 0xFF) << 24

		ecx += edx // Increase key by data value
		edx ^= ecx // XOR data with updated key

		// Write back in little-endian format
		raw[pos] = byte(edx & 0xFF)
		raw[pos+1] = byte((edx >> 8) & 0xFF)
		raw[pos+2] = byte((edx >> 16) & 0xFF)
		raw[pos+3] = byte((edx >> 24) & 0xFF)
		pos += 4
	}

	// Write final key to last 4 bytes
	raw[pos] = byte(ecx & 0xFF)
	raw[pos+1] = byte((ecx >> 8) & 0xFF)
	raw[pos+2] = byte((ecx >> 16) & 0xFF)
	raw[pos+3] = byte((ecx >> 24) & 0xFF)
}

// DecXORPass reverses XOR pass encryption for static packets
// Algorithm: given encrypted data with XOR key in last 4 bytes,
// reverse the encryption process to get original data
func DecXORPass(raw []byte, offset int, size int) {
	if size <= 8 {
		return // Not enough data for XOR pass
	}

	stop := offset + size - 8
	pos := 4 + offset

	// Read the final key from last 4 bytes
	keyPos := offset + size - 4
	finalKey := uint32(raw[keyPos]) & 0xFF
	finalKey |= (uint32(raw[keyPos+1]) & 0xFF) << 8
	finalKey |= (uint32(raw[keyPos+2]) & 0xFF) << 16
	finalKey |= (uint32(raw[keyPos+3]) & 0xFF) << 24

	// To reverse the process, we need to work from the beginning
	// because each key depends on the previous decrypted data

	// First, we need to find the initial key
	// This requires simulating the forward process to determine what key would produce finalKey

	// For now, let's try a simpler approach:
	// Since encryption does: key += data; data ^= key
	// Decryption should do: data ^= key; key -= original_data

	// But we don't know the initial key, so we need to work backwards
	// from the final key to determine the initial key

	// Store encrypted blocks first
	encryptedBlocks := make([]uint32, 0)
	for i := pos; i < stop; i += 4 {
		block := uint32(raw[i]) & 0xFF
		block |= (uint32(raw[i+1]) & 0xFF) << 8
		block |= (uint32(raw[i+2]) & 0xFF) << 16
		block |= (uint32(raw[i+3]) & 0xFF) << 24
		encryptedBlocks = append(encryptedBlocks, block)
	}

	// Reverse the process: work backwards from final key
	currentKey := finalKey
	for i := len(encryptedBlocks) - 1; i >= 0; i-- {
		// encrypted_data = original_data ^ current_key
		// So: original_data = encrypted_data ^ current_key
		originalData := encryptedBlocks[i] ^ currentKey

		// In forward process: current_key = previous_key + original_data
		// So: previous_key = current_key - original_data
		currentKey -= originalData

		// Write back the original data
		writePos := 4 + offset + i*4
		raw[writePos] = byte(originalData & 0xFF)
		raw[writePos+1] = byte((originalData >> 8) & 0xFF)
		raw[writePos+2] = byte((originalData >> 16) & 0xFF)
		raw[writePos+3] = byte((originalData >> 24) & 0xFF)
	}

	// Clear the key bytes (set to 0)
	for i := 0; i < 4; i++ {
		raw[keyPos+i] = 0
	}
}
