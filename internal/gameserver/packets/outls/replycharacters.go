package outls

import "github.com/VerTox/l2go/pkg/l2pkt"

type ReplyCharacters struct {
	account    string
	charCount  int
	charsInDel int
}

func NewReplyCharacters(account string, charCount, charsInDel int) *ReplyCharacters {
	return &ReplyCharacters{
		account:    account,
		charCount:  charCount,
		charsInDel: charsInDel,
	}
}

func (rc *ReplyCharacters) GetData() []byte {
	buffer := l2pkt.NewWriter()
	buffer.WriteC(0x08)       // Packet type: ReplyCharacters
	buffer.WriteS(rc.account) // UTF-16LE string
	buffer.WriteC(byte(rc.charCount))
	buffer.WriteC(byte(rc.charsInDel))

	// L2J writes one deletion timestamp (writeQ) per character pending deletion, and the
	// LoginServer reads exactly charsInDel of them — so we MUST write that many to keep
	// the stream aligned. We don't plumb the real delete_time values yet, so send zeros.
	// The char-select deletion countdown is driven by the game server's CharSelectionInfo,
	// not this packet; real timestamps here are a minor follow-up. (l2go-rx4)
	for i := 0; i < rc.charsInDel; i++ {
		buffer.WriteQ(0)
	}

	return buffer.Bytes()
}
