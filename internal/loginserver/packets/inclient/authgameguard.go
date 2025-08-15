package inclient

import (
	"fmt"

	"github.com/VerTox/l2go/internal/loginserver/packets"
)

type AuthGameGuard struct {
	SessionId uint32
	Data1     uint32
	Data2     uint32
	Data3     uint32
	Data4     uint32
}

func NewAuthGameGuard(data []byte) *AuthGameGuard {
	reader := packets.NewReader(data)

	return &AuthGameGuard{
		SessionId: reader.ReadUInt32(),
		Data1:     reader.ReadUInt32(),
		Data2:     reader.ReadUInt32(),
		Data3:     reader.ReadUInt32(),
		Data4:     reader.ReadUInt32(),
	}
}

func (agg *AuthGameGuard) GetSessionId() uint32 {
	return agg.SessionId
}

func (agg *AuthGameGuard) String() string {
	return fmt.Sprintf("AuthGameGuard{SessionId: %08X, Data1: %08X, Data2: %08X, Data3: %08X, Data4: %08X}",
		agg.SessionId, agg.Data1, agg.Data2, agg.Data3, agg.Data4)
}
