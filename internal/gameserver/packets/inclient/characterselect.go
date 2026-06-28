package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

type CharacterSelect struct {
	CharID int32
}

func (p *CharacterSelect) Read(r *l2pkt.Reader) bool {
	charID, err := r.ReadD()
	if err != nil {
		return false
	}

	p.CharID = charID

	r.ReadH()
	r.ReadD()
	r.ReadD()
	r.ReadD()
	return true
}
