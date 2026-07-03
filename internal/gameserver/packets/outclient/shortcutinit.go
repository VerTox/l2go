package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

type ShortCut struct {
	Slot             int32
	Page             int32
	Type             ShortCutType
	ID               int32
	Level            int32
	CharacterType    int32 // 0 - player, 1 - pet
	SharedReuseGroup int32 //always -1
}

type ShortCutType int32

const (
	ShortCutTypeNone ShortCutType = iota
	ShortCutTypeItem
	ShortCutTypeSkill
	ShortCutTypeAction
	ShortCutTypeMacro
	ShortCutTypeRecipe
	ShortCutTypeBookMark
)

func BuildShortCutInit(shortcuts []ShortCut) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x45)
	w.WriteD(int32(len(shortcuts)))
	for _, sc := range shortcuts {
		w.WriteD(int32(sc.Type))
		w.WriteD(sc.Slot + (sc.Page * 12))
		switch sc.Type {
		case ShortCutTypeItem:
			w.WriteD(sc.ID)
			w.WriteD(0x01)
			w.WriteD(sc.SharedReuseGroup)
			w.WriteD(0x00)
			w.WriteD(0x00)
			w.WriteH(0x00) // L2J writes two H here, NOT a trailing D pair
			w.WriteH(0x00)
		case ShortCutTypeSkill:
			w.WriteD(sc.ID)
			w.WriteD(sc.Level)
			w.WriteD(0x00)
			w.WriteD(0x01)
		case ShortCutTypeAction:
		case ShortCutTypeMacro:
		case ShortCutTypeRecipe:
		case ShortCutTypeBookMark:
			w.WriteD(sc.ID)
			w.WriteD(0x01)
		default:
			continue
		}
	}
	return w.Bytes()
}
