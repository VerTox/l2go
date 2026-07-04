package inclient

import "testing"

func TestParseAttack(t *testing.T) {
	// objectId=8, origin=(1,2,3), attackId=0  → cddddc little-endian
	payload := []byte{8, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 0}
	p, err := ParseAttack(payload)
	if err != nil {
		t.Fatal(err)
	}
	if p.ObjectID != 8 || p.OriginX != 1 || p.OriginY != 2 || p.OriginZ != 3 || p.AttackID != 0 {
		t.Fatalf("bad parse: %+v", p)
	}
}
