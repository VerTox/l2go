package outclient

import (
	"bytes"
	"testing"
)

// TestCreatureSayBytes проверяет байтовую раскладку CreatureSay (0x4a) для
// обычного текста игрока: writeC(0x4a), writeD(objectId), writeD(type),
// writeS(charName), writeD(npcStringId=-1), writeS(text).
func TestCreatureSayBytes(t *testing.T) {
	got := BuildCreatureSay(0x10203040, ChatTell, "A", "B")
	want := []byte{
		0x4a,                   // opcode
		0x40, 0x30, 0x20, 0x10, // objectId (LE)
		0x02, 0x00, 0x00, 0x00, // chatType = TELL
		0x41, 0x00, 0x00, 0x00, // "A" + null terminator (UTF-16LE)
		0xff, 0xff, 0xff, 0xff, // npcStringId = -1
		0x42, 0x00, 0x00, 0x00, // "B" + null terminator (UTF-16LE)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("CreatureSay bytes mismatch\n got: %x\nwant: %x", got, want)
	}
}

func TestCreatureSayGolden(t *testing.T) {
	checkGolden(t, "creaturesay_player", BuildCreatureSay(268437456, ChatAll, "Hero", "Hello world"))
}
