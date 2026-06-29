package client

import (
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// toCharSelectInfoPackage must produce a fully-populated lobby entry, including
// the paperdoll and appearance fields. A regression here left the paperdoll empty
// when returning to the lobby from the world (restart path, see l2go-5bn).
func TestToCharSelectInfoPackage(t *testing.T) {
	entry := models.CharacterListEntry{
		Character: models.Character{
			Name:       "Tester",
			ID:         42,
			Level:      5,
			Experience: 4461,
			HairStyle:  3,
			HairColor:  1,
			Face:       2,
		},
		PaperdollItems: []models.CharacterItem{
			{ItemID: 1234, LocData: int(models.SlotChest)},
			{ItemID: 5678, LocData: int(models.SlotRHand)},
		},
	}

	pkg := toCharSelectInfoPackage(entry)

	if pkg.Name != "Tester" || pkg.ObjectID != 42 {
		t.Fatalf("identity not copied: name=%q id=%d", pkg.Name, pkg.ObjectID)
	}
	if pkg.Level != 5 || pkg.Exp != 4461 {
		t.Errorf("level/exp not copied: level=%d exp=%d", pkg.Level, pkg.Exp)
	}
	if pkg.HairStyle != 3 || pkg.HairColor != 1 || pkg.Face != 2 {
		t.Errorf("appearance not copied: hair=%d/%d face=%d", pkg.HairStyle, pkg.HairColor, pkg.Face)
	}
	if len(pkg.PaperdollItemIDs) != 26 {
		t.Fatalf("paperdoll array size = %d, want 26", len(pkg.PaperdollItemIDs))
	}
	// chest → packet index 10, rhand → packet index 7
	if pkg.PaperdollItemIDs[10] != 1234 {
		t.Errorf("chest item missing: got %d, want 1234", pkg.PaperdollItemIDs[10])
	}
	if pkg.PaperdollItemIDs[7] != 5678 {
		t.Errorf("rhand item missing: got %d, want 5678", pkg.PaperdollItemIDs[7])
	}
}
