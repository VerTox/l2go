package ings

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type PlayerTracert struct {
	account string
	pcIP    string
	hop1    string
	hop2    string
	hop3    string
	hop4    string
}

func NewPlayerTracert(data []byte) *PlayerTracert {
	if len(data) < 2 {
		// Packet too short
		return nil
	}

	reader := packets.NewReader(data)

	// Read account name and traceroute info (all UTF-16LE strings)
	account := reader.ReadString()
	pcIP := reader.ReadString()
	hop1 := reader.ReadString()
	hop2 := reader.ReadString()
	hop3 := reader.ReadString()
	hop4 := reader.ReadString()

	return &PlayerTracert{
		account: account,
		pcIP:    pcIP,
		hop1:    hop1,
		hop2:    hop2,
		hop3:    hop3,
		hop4:    hop4,
	}
}

func (pt *PlayerTracert) GetAccount() string {
	return pt.account
}

func (pt *PlayerTracert) GetPCIP() string {
	return pt.pcIP
}

func (pt *PlayerTracert) GetHops() []string {
	return []string{pt.hop1, pt.hop2, pt.hop3, pt.hop4}
}
