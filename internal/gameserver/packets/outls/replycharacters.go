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

	return buffer.Bytes()
}
