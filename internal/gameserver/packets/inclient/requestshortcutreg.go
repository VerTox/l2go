package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

type ShortcutRegisterRequest struct {
	Type          ShortCutType
	Id            int32
	Slot          int32
	Page          int32
	Level         int32
	CharacterType int32
}

func (p *ShortcutRegisterRequest) Read(r *l2pkt.Reader) bool {
	t, err := r.ReadD()
	if err != nil {
		return false
	}
	p.Type = ShortCutType(t)

	slot, err := r.ReadD()
	if err != nil {
		return false
	}
	p.Slot = slot % 12
	p.Page = slot / 12

	id, err := r.ReadD()
	if err != nil {
		return false
	}
	p.Id = id

	level, err := r.ReadD()
	if err != nil {
		return false
	}
	p.Level = level

	charType, err := r.ReadD()
	if err != nil {
		return false
	}
	p.CharacterType = charType

	return true
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
