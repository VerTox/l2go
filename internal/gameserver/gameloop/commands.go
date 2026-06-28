package gameloop

import "github.com/VerTox/l2go/internal/gameserver/models"

// Command is sent from client handler goroutines to the game loop.
type Command interface {
	commandMarker()
}

// CmdAttackRequest — player wants to attack a target.
type CmdAttackRequest struct {
	AttackerCharID  int32
	TargetObjectID  int32
	AttackerPos     models.Position
	AccountName     string
}

func (CmdAttackRequest) commandMarker() {}

// CmdCancelAttack — player cancelled attack (moved, pressed Esc, etc.).
type CmdCancelAttack struct {
	CharID int32
}

func (CmdCancelAttack) commandMarker() {}

// CmdPlayerDisconnected — player disconnected from the game.
type CmdPlayerDisconnected struct {
	CharID int32
}

func (CmdPlayerDisconnected) commandMarker() {}

// CmdPlayerEnteredWorld — player finished loading into the game world.
type CmdPlayerEnteredWorld struct {
	CharID      int32
	AccountName string
	Position    models.Position
}

func (CmdPlayerEnteredWorld) commandMarker() {}

// CmdPlayerMoved — player position changed (for active region tracking).
type CmdPlayerMoved struct {
	CharID   int32
	Position models.Position
}

func (CmdPlayerMoved) commandMarker() {}
