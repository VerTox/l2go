package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildDie builds the Die packet (0x00).
// Sent when an NPC or player dies.
// The 7 int32 flags control what actions are available (to town, to clan hall,
// to castle, sweep, fixed res, to fortress, to hideout). All 0 for NPCs.
func BuildDie(objectID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x00) // Die opcode
	w.WriteD(objectID)
	// 7 flags — all zero for NPC death (player death flags set separately)
	w.WriteD(0) // toVillage
	w.WriteD(0) // toClanHall
	w.WriteD(0) // toCastle
	w.WriteD(0) // toSiege HQ / sweep
	w.WriteD(0) // isFixedRes
	w.WriteD(0) // toFortress
	w.WriteD(0) // toHideout
	return w.Bytes()
}

// BuildPlayerDie builds the Die packet for a player death.
// toVillage=1 allows the player to resurrect in the nearest village.
func BuildPlayerDie(objectID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x00) // Die opcode
	w.WriteD(objectID)
	w.WriteD(1) // toVillage = yes
	w.WriteD(0) // toClanHall
	w.WriteD(0) // toCastle
	w.WriteD(0) // toSiege HQ / sweep
	w.WriteD(0) // isFixedRes
	w.WriteD(0) // toFortress
	w.WriteD(0) // toHideout
	return w.Bytes()
}
