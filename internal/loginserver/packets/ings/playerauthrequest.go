package ings

import (
	"github.com/VerTox/l2go/internal/loginserver/models"
	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type PlayerAuthRequest struct {
	account   string
	playKey1  uint32
	playKey2  uint32
	loginKey1 uint32
	loginKey2 uint32
}

func NewPlayerAuthRequest(data []byte) *PlayerAuthRequest {
	if len(data) < 18 { // at least account (2 bytes min) + 4 keys (16 bytes)
		// Packet too short
		return nil
	}

	reader := packets.NewReader(data)

	// Read account name (UTF-16LE string)
	account := reader.ReadString()

	// Read session keys (4 x uint32)
	playKey1 := reader.ReadUInt32()
	playKey2 := reader.ReadUInt32()
	loginKey1 := reader.ReadUInt32()
	loginKey2 := reader.ReadUInt32()

	return &PlayerAuthRequest{
		account:   account,
		playKey1:  playKey1,
		playKey2:  playKey2,
		loginKey1: loginKey1,
		loginKey2: loginKey2,
	}
}

func (par *PlayerAuthRequest) GetAccount() string {
	return par.account
}

func (par *PlayerAuthRequest) GetSessionKey() models.SessionKey {
	return models.SessionKey{
		LoginKey1: par.loginKey1,
		LoginKey2: par.loginKey2,
		PlayKey1:  par.playKey1,
		PlayKey2:  par.playKey2,
	}
}
