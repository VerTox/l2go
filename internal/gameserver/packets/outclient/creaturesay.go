package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// Chat channel types (L2J Say2 / High Five). Valid client-sendable range is
// [ChatAll, ChatMPCCRoom] — see Say2 CHAT_NAMES.length validation.
const (
	ChatAll                = 0
	ChatShout              = 1 // "!"
	ChatTell               = 2
	ChatParty              = 3 // "#"
	ChatClan               = 4 // "@"
	ChatGM                 = 5
	ChatPetitionPlayer     = 6
	ChatPetitionGM         = 7
	ChatTrade              = 8 // "+"
	ChatAlliance           = 9 // "$"
	ChatAnnouncement       = 10
	ChatBoat               = 11
	ChatL2Friend           = 12
	ChatMSN                = 13
	ChatPartymatchRoom     = 14
	ChatPartyroomCommander = 15
	ChatPartyroomAll       = 16
	ChatHeroVoice          = 17
	ChatCriticalAnnounce   = 18
	ChatScreenAnnounce     = 19
	ChatBattlefield        = 20
	ChatMPCCRoom           = 21

	// ChatMaxValidType is the highest chat type the client may send (CHAT_NAMES
	// has 22 entries, indices 0..21). Types 22/23 (NPC_ALL/NPC_SHOUT) are
	// server-internal and rejected on the Say2 ingress like in L2J.
	ChatMaxValidType = ChatMPCCRoom
)

// creatureSayNpcString is the npcStringId field for ordinary (non-NpcString)
// chat text: -1 (0xFFFFFFFF) tells the client to use the literal text string.
const creatureSayNpcString int32 = -1

// BuildCreatureSay builds a CreatureSay packet (opcode 0x4a) carrying a plain
// text line from a named speaker (a player). Layout matches L2J CreatureSay for
// the charName+text case: writeC(0x4a), writeD(objectId), writeD(chatType),
// writeS(charName), writeD(npcStringId=-1), writeS(text).
func BuildCreatureSay(objectID int32, chatType int32, charName, text string) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x4a)
	w.WriteD(objectID)
	w.WriteD(chatType)
	w.WriteS(charName)
	w.WriteD(creatureSayNpcString)
	w.WriteS(text)
	return w.Bytes()
}
