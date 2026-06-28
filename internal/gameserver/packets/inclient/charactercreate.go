package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// CharacterCreate (opcode 0x0c) - client creates a new character.
type CharacterCreate struct {
	Name      string
	Race      int32
	Sex       int32
	ClassID   int32
	HairStyle int32
	HairColor int32
	Face      int32
	STR       int32
	DEX       int32
	CON       int32
	INT       int32
	WIT       int32
	MEN       int32
}

func (p *CharacterCreate) Read(r *l2pkt.Reader) bool {
	name, err := r.ReadS()
	if err != nil {
		return false
	}
	p.Name = name

	race, err := r.ReadD()
	if err != nil {
		return false
	}
	p.Race = race

	sex, err := r.ReadD()
	if err != nil {
		return false
	}
	p.Sex = sex

	classID, err := r.ReadD()
	if err != nil {
		return false
	}
	p.ClassID = classID

	hairStyle, err := r.ReadD()
	if err != nil {
		return false
	}
	p.HairStyle = hairStyle

	hairColor, err := r.ReadD()
	if err != nil {
		return false
	}
	p.HairColor = hairColor

	face, err := r.ReadD()
	if err != nil {
		return false
	}
	p.Face = face

	// Starting stats
	str, err := r.ReadD()
	if err != nil {
		return false
	}
	p.STR = str

	dex, err := r.ReadD()
	if err != nil {
		return false
	}
	p.DEX = dex

	con, err := r.ReadD()
	if err != nil {
		return false
	}
	p.CON = con

	intStat, err := r.ReadD()
	if err != nil {
		return false
	}
	p.INT = intStat

	wit, err := r.ReadD()
	if err != nil {
		return false
	}
	p.WIT = wit

	men, err := r.ReadD()
	if err != nil {
		return false
	}
	p.MEN = men

	return true
}

func NewCharacterCreate(_ []byte) *CharacterCreate { return &CharacterCreate{} }