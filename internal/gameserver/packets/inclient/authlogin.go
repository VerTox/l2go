package inclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// AuthLogin represents client request to enter GameServer (opcode 0x2b).
// This packet authenticates the client to the GameServer using session keys.
type AuthLogin struct {
	Account   string
	PlayKey1  int32
	PlayKey2  int32
	LoginKey1 int32
	LoginKey2 int32
}

func (p *AuthLogin) Read(r *l2pkt.Reader) bool {
	account, err := r.ReadS()
	if err != nil {
		return false
	}

	p.Account = account

	playKey2, err := r.ReadD()
	if err != nil {
		return false
	}

	p.PlayKey2 = playKey2

	playKey1, err := r.ReadD()
	if err != nil {
		return false
	}

	p.PlayKey1 = playKey1

	loginKey1, err := r.ReadD()
	if err != nil {
		return false
	}

	p.LoginKey1 = loginKey1
	loginKey2, err := r.ReadD()
	if err != nil {
		return false
	}

	p.LoginKey2 = loginKey2

	return true
}

func NewAuthLogin(_ []byte) *AuthLogin { return &AuthLogin{} }
