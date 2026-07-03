package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildShortCutRegister builds ShortCutRegister (0x44) — the server echo sent in
// response to RequestShortCutReg, confirming a single shortcut placement.
// Layout mirrors L2J HF ShortCutRegister.writeImpl (note: differs from ShortCutInit
// per-item — ITEM uses characterType and ends with a D augment id, not two H).
func BuildShortCutRegister(sc ShortCut) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x44)
	w.WriteD(int32(sc.Type))
	w.WriteD(sc.Slot + (sc.Page * 12))
	switch sc.Type {
	case ShortCutTypeItem:
		w.WriteD(sc.ID)
		w.WriteD(sc.CharacterType)
		w.WriteD(sc.SharedReuseGroup)
		w.WriteD(0x00) // unknown
		w.WriteD(0x00) // unknown
		w.WriteD(0x00) // item augment id
	case ShortCutTypeSkill:
		w.WriteD(sc.ID)
		w.WriteD(sc.Level)
		w.WriteC(0x00) // C5
		w.WriteD(sc.CharacterType)
	case ShortCutTypeAction, ShortCutTypeMacro, ShortCutTypeRecipe, ShortCutTypeBookMark:
		w.WriteD(sc.ID)
		w.WriteD(sc.CharacterType)
	default:
	}
	return w.Bytes()
}
