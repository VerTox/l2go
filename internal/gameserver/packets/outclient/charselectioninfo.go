package outclient

import (
	"time"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// CharSelectInfoPackage — «снимок» чара для экрана выбора.
type CharSelectInfoPackage struct {
	Name        string
	ObjectID    int32
	ClanID      int32
	Sex         int32
	Race        int32
	BaseClassID int32
	ClassID     int32

	X, Y, Z int32

	CurrentHp float64
	CurrentMp float64
	MaxHp     float64
	MaxMp     float64

	Sp    int32
	Exp   int64
	Level int32

	Karma    int32
	PkKills  int32
	PvPKills int32

	// бумажная кукла — последовательность itemId’ов в порядке, ожидаемом клиентом
	PaperdollItemIDs []int32

	HairStyle int32
	HairColor int32
	Face      int32

	DeleteTimerMs  int64 // millis; 0 если не помечен к удалению
	AugmentationID int32
	EnchantEffect  int32 // capped до 127
	VitalityPoints int32

	LastAccessMs int64 // millis unix
}

// Paperdoll заказ (HF). Если удобнее — формируй сразу готовый срез itemId’ов в этом порядке.
var PaperdollOrder = []string{
	"under", "rear", "lear", "neck", "rfinger", "lfinger", "head", "rhand", "lhand",
	"gloves", "chest", "legs", "feet", "back", "lrhand", "hair", "hair2", "rbracelet",
	"lbracelet", "deco1", "deco2", "deco3", "deco4", "deco5", "deco6", "belt",
}

// Вспомогательная заглушка конфигурации аккаунта (макс. число персонажей).
type CharacterConfig struct {
	CharMaxNumber int32
}

type CharSelectionInfo struct {
	LoginName string
	SessionID int32
	ActiveIdx int // -1 => выбрать по LastAccess
	Chars     []CharSelectInfoPackage
	CharConf  CharacterConfig
}

// процент прогресса уровня — заглушка; подставишь свою формулу/таблицу
func percentFromCurrentLevel(exp int64, level int32) float64 {
	// TODO: заменить на реальную логику ExperienceData HF
	return 0.0
}

// Write сериализует пакет в payload (начиная с opcode).
func (p CharSelectionInfo) Write(w *l2pkt.Writer) {
	w.WriteC(0x09)
	size := int32(len(p.Chars))
	w.WriteD(size)

	// Can prevent players from creating new characters
	w.WriteD(p.CharConf.CharMaxNumber)
	w.WriteC(0x00) // reserved

	active := p.ActiveIdx
	if active == -1 && len(p.Chars) > 0 {
		var last int64
		for i := range p.Chars {
			if p.Chars[i].LastAccessMs > last {
				last = p.Chars[i].LastAccessMs
				active = i
			}
		}
	}

	now := time.Now().UnixMilli()

	for i := 0; i < int(size); i++ {
		c := p.Chars[i]

		w.WriteS(c.Name)
		w.WriteD(c.ObjectID)
		w.WriteS(p.LoginName)
		w.WriteD(p.SessionID)
		w.WriteD(c.ClanID)
		w.WriteD(0x00) // Builder Level

		w.WriteD(c.Sex)
		w.WriteD(c.Race)
		w.WriteD(c.BaseClassID)

		w.WriteD(0x01) // active ??

		w.WriteD(c.X)
		w.WriteD(c.Y)
		w.WriteD(c.Z)

		w.WriteF(c.CurrentHp)
		w.WriteF(c.CurrentMp)

		w.WriteD(c.Sp)
		w.WriteQ(c.Exp)
		w.WriteF(percentFromCurrentLevel(c.Exp, c.Level))

		w.WriteD(c.Level)

		w.WriteD(c.Karma)
		w.WriteD(c.PkKills)
		w.WriteD(c.PvPKills)

		// 7 reserved ints
		for k := 0; k < 7; k++ {
			w.WriteD(0)
		}

		// paperdoll itemIds в ожидаемом порядке
		for _, itemID := range c.PaperdollItemIDs {
			w.WriteD(itemID)
		}

		w.WriteD(c.HairStyle)
		w.WriteD(c.HairColor)
		w.WriteD(c.Face)

		w.WriteF(c.MaxHp)
		w.WriteF(c.MaxMp)

		// секунды до удаления (если помечен)
		secsLeft := int32(0)
		if c.DeleteTimerMs > 0 {
			d := (c.DeleteTimerMs - now) / 1000
			if d > 0 {
				secsLeft = int32(d)
			}
		}
		w.WriteD(secsLeft)

		w.WriteD(c.ClassID)
		if i == active {
			w.WriteD(0x01)
		} else {
			w.WriteD(0x00)
		}

		enc := c.EnchantEffect
		if enc > 127 {
			enc = 127
		}
		w.WriteC(byte(enc))
		w.WriteD(c.AugmentationID)

		w.WriteD(0x00) // no transform on charselect

		// Pet stub
		w.WriteD(0x00) // Pet ID
		w.WriteD(0x00) // Pet Level
		w.WriteD(0x00) // Pet Max Food
		w.WriteD(0x00) // Pet Current Food
		w.WriteF(0.0)  // Pet Max HP
		w.WriteF(0.0)  // Pet Max MP

		// High Five Vitality
		w.WriteD(c.VitalityPoints)
	}
}
