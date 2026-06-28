package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// CharSelected packet (opcode 0x0b) - confirms character selection and starts world entry
// Matches Java L2J CharSelected.writeImpl() byte-for-byte
type CharSelected struct {
	Name      string
	ObjectID  int32
	Title     string
	SessionID int32
	ClanID    int32
	Sex       int32
	Race      int32
	ClassID   int32
	X         int32
	Y         int32
	Z         int32
	CurrentHP float64
	CurrentMP float64
	SP        int32
	EXP       int64
	Level     int32
	Karma     int32
	PkKills   int32
	// Attributes in Java L2J order: INT, STR, CON, MEN, DEX, WIT
	INT int32
	STR int32
	CON int32
	MEN int32
	DEX int32
	WIT int32
	// Game time (minutes in day cycle)
	GameTime int32
}

// BuildCharSelected creates CharSelected packet data matching Java L2J structure
func BuildCharSelected(char CharSelected) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x0b) // CharSelected opcode

	// Character identification
	w.WriteS(char.Name)
	w.WriteD(char.ObjectID)
	w.WriteS(char.Title)
	w.WriteD(char.SessionID)

	// Clan info
	w.WriteD(char.ClanID)
	w.WriteD(0) // unknown field (Java L2J writes 0x00)

	// Physical attributes
	w.WriteD(char.Sex)
	w.WriteD(char.Race)
	w.WriteD(char.ClassID)

	// Active flag
	w.WriteD(1) // always 1 (active)

	// Position
	w.WriteD(char.X)
	w.WriteD(char.Y)
	w.WriteD(char.Z)

	// Current stats
	w.WriteF(char.CurrentHP)
	w.WriteF(char.CurrentMP)
	w.WriteD(char.SP)  // 4 bytes (Java writeD)
	w.WriteQ(char.EXP) // 8 bytes (Java writeQ)
	w.WriteD(char.Level)
	w.WriteD(char.Karma)
	w.WriteD(char.PkKills)

	// Base stats in Java L2J order: INT, STR, CON, MEN, DEX, WIT
	w.WriteD(char.INT)
	w.WriteD(char.STR)
	w.WriteD(char.CON)
	w.WriteD(char.MEN)
	w.WriteD(char.DEX)
	w.WriteD(char.WIT)

	// Game time and unknown
	w.WriteD(char.GameTime)
	w.WriteD(0) // unknown

	// Duplicate ClassID (Java L2J writes classId twice)
	w.WriteD(char.ClassID)

	// 4x zero DWORDs (padding/reserved)
	w.WriteD(0)
	w.WriteD(0)
	w.WriteD(0)
	w.WriteD(0)

	// 64 bytes of zero padding
	w.WriteB(make([]byte, 64))

	// Final zero DWORD
	w.WriteD(0)

	return w.Bytes()
}
