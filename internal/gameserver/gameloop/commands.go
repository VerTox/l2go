package gameloop

import "github.com/VerTox/l2go/internal/gameserver/models"

// Command is sent from client handler goroutines to the game loop.
type Command interface {
	commandMarker()
}

// CmdAttackRequest — player wants to attack a target.
type CmdAttackRequest struct {
	AttackerCharID int32
	TargetObjectID int32
	AttackerPos    models.Position
	AccountName    string
}

func (CmdAttackRequest) commandMarker() {}

// CmdInteractRequest — player clicked a non-attackable NPC out of interaction
// range; approach it and open the dialogue on arrival (L2J AI_INTENTION_INTERACT).
type CmdInteractRequest struct {
	CharID         int32
	TargetObjectID int32
	AccountName    string
}

func (CmdInteractRequest) commandMarker() {}

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

// CmdMoveToLocation — player issued a ground move (clicked the ground). Cancels any
// attack/interact intention so the loop stops chasing the previous target.
type CmdMoveToLocation struct {
	CharID int32
}

func (CmdMoveToLocation) commandMarker() {}

// CmdTeleport — relocate a player to a new position. The loop broadcasts
// TeleportToLocation + DeleteObject (decay), moves the player and flags it teleporting;
// visibility at the destination is re-established when the client sends Appearing.
type CmdTeleport struct {
	CharID  int32
	Dest    models.Position
	Heading int32
}

func (CmdTeleport) commandMarker() {}
