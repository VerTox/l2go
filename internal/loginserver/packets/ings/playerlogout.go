package ings

import (
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type PlayerLogout struct {
	account string
}

func NewPlayerLogout(data []byte) *PlayerLogout {
	if len(data) < 2 { // at least account name (2 bytes minimum for empty string)
		// Packet too short
		return nil
	}

	reader := packets.NewReader(data)

	// Read account name (UTF-16LE string)
	account := reader.ReadString()

	return &PlayerLogout{
		account: account,
	}
}

func (pl *PlayerLogout) GetAccount() string {
	return pl.account
}
