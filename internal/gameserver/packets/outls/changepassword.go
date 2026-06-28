package outls

import "github.com/VerTox/l2go/pkg/l2pkt"

type ChangePassword struct {
	accountName   string
	characterName string
	oldPassword   string
	newPassword   string
}

func NewChangePassword(accountName, characterName, oldPassword, newPassword string) *ChangePassword {
	return &ChangePassword{
		accountName:   accountName,
		characterName: characterName,
		oldPassword:   oldPassword,
		newPassword:   newPassword,
	}
}

func (cp *ChangePassword) GetData() []byte {
	buffer := l2pkt.NewWriter()
	buffer.WriteC(0x0B)             // Packet type: ChangePassword
	buffer.WriteS(cp.accountName)   // UTF-16LE string
	buffer.WriteS(cp.characterName) // UTF-16LE string
	buffer.WriteS(cp.oldPassword)   // UTF-16LE string
	buffer.WriteS(cp.newPassword)   // UTF-16LE string

	return buffer.Bytes()
}
