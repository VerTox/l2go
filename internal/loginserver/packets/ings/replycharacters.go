package ings

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

// ReplyCharacters packet received from GameServer with character count information
// Opcode: 0x08
type ReplyCharacters struct {
	Account        string
	CharacterCount int
	DeletionCount  int
	DeletionTimes  []int64 // timestamps for characters pending deletion
}

func NewReplyCharacters(data []byte) *ReplyCharacters {
	if len(data) < 3 {
		return nil
	}

	reader := packets.NewReader(data)

	// Read account name (UTF-16 string with null terminator)
	account := reader.ReadString()
	if account == "" {
		return nil
	}

	// Read character count (1 byte)
	charCount := reader.ReadUInt8()

	// Read deletion count (1 byte)
	deletionCount := reader.ReadUInt8()

	// Read deletion timestamps (8 bytes each)
	deletionTimes := make([]int64, deletionCount)
	for i := 0; i < int(deletionCount); i++ {
		timestamp := int64(reader.ReadUInt64())
		deletionTimes[i] = timestamp
	}

	return &ReplyCharacters{
		Account:        account,
		CharacterCount: int(charCount),
		DeletionCount:  int(deletionCount),
		DeletionTimes:  deletionTimes,
	}
}

func (rc *ReplyCharacters) GetAccount() string {
	return rc.Account
}

func (rc *ReplyCharacters) GetCharacterCount() int {
	return rc.CharacterCount
}

func (rc *ReplyCharacters) GetDeletionTimes() []int64 {
	return rc.DeletionTimes
}
