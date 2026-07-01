package registry

import (
	"context"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

func TestPlayerWorldState_SnapshotCharacter(t *testing.T) {
	t.Run("applies live position/heading and returns an independent copy", func(t *testing.T) {
		char := &models.Character{ID: 7, Level: 3, Experience: 1000, SP: 50}
		char.SetPosition(1, 2, 3)

		state := &PlayerWorldState{
			CharID:    7,
			Character: char,
			Position:  models.Position{X: 100, Y: 200, Z: 300},
			Heading:   45,
		}

		snap, ok := state.SnapshotCharacter()
		if !ok {
			t.Fatal("expected ok=true for non-nil character")
		}

		// Live world position/heading must be baked into the snapshot.
		if snap.Position != (models.Position{X: 100, Y: 200, Z: 300}) {
			t.Errorf("snapshot position = %+v, want live world position", snap.Position)
		}
		if snap.Heading != 45 {
			t.Errorf("snapshot heading = %d, want 45", snap.Heading)
		}
		if snap.Experience != 1000 || snap.SP != 50 || snap.Level != 3 {
			t.Errorf("snapshot progress fields not copied: %+v", snap)
		}

		// Mutating the live character after the snapshot must NOT change the snapshot
		// (it is a value copy — this is what makes off-loop persistence race-free).
		char.Experience = 9999
		char.Level = 99
		if snap.Experience != 1000 || snap.Level != 3 {
			t.Errorf("snapshot changed after mutating original: exp=%d level=%d", snap.Experience, snap.Level)
		}
	})

	t.Run("nil character returns ok=false", func(t *testing.T) {
		state := &PlayerWorldState{CharID: 7, Character: nil}
		if _, ok := state.SnapshotCharacter(); ok {
			t.Error("expected ok=false for nil character")
		}
	})
}

func TestWorldRegistry_SnapshotOnlineCharacters(t *testing.T) {
	wr := NewWorldRegistry()
	ctx := context.Background()

	_ = wr.AddPlayer(ctx, &models.Character{ID: 1, Name: "A", Experience: 100})
	_ = wr.AddPlayer(ctx, &models.Character{ID: 2, Name: "B", Experience: 200})
	_ = wr.UpdatePlayerPosition(ctx, 1, models.Position{X: 10, Y: 20, Z: 30}, 90)

	snaps := wr.SnapshotOnlineCharacters()
	if len(snaps) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(snaps))
	}

	byID := map[int32]models.Character{}
	for _, c := range snaps {
		byID[c.ID] = c
	}
	if byID[1].Position != (models.Position{X: 10, Y: 20, Z: 30}) || byID[1].Heading != 90 {
		t.Errorf("char 1 snapshot did not reflect updated position/heading: %+v", byID[1])
	}
	if byID[2].Experience != 200 {
		t.Errorf("char 2 exp = %d, want 200", byID[2].Experience)
	}
}
