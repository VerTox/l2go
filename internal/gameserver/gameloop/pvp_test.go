package gameloop

import (
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

func TestCanAttackPlayer(t *testing.T) {
	now := time.Unix(1000, 0)
	clean := func() *registry.PlayerWorldState {
		return &registry.PlayerWorldState{Character: &models.Character{}}
	}

	// Clean target, no ctrl → blocked.
	if allowed, _ := canAttackPlayer(clean(), false, now); allowed {
		t.Fatal("clean target without ctrl must be blocked")
	}
	// Clean target, ctrl → allowed and attacker flags.
	if allowed, flag := canAttackPlayer(clean(), true, now); !allowed || !flag {
		t.Fatalf("ctrl on clean target: want allowed+flag, got %v %v", allowed, flag)
	}
	// Flagged target → allowed, no attacker flag.
	flagged := clean()
	flagged.PvPFlagUntil = now.Add(time.Minute)
	if allowed, flag := canAttackPlayer(flagged, false, now); !allowed || flag {
		t.Fatalf("flagged target: want allowed, no flag, got %v %v", allowed, flag)
	}
	// PK target (karma>0) → allowed, no attacker flag.
	pk := clean()
	pk.Character.Karma = 100
	if allowed, flag := canAttackPlayer(pk, false, now); !allowed || flag {
		t.Fatalf("PK target: want allowed, no flag, got %v %v", allowed, flag)
	}
}
