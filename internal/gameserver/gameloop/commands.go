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

// CmdCastRequest — player wants to cast a skill (RequestMagicSkillUse). The loop
// resolves the level from the caster's KnownSkills and the template from SkillData.
type CmdCastRequest struct {
	CasterCharID int32
	SkillID      int32
	CtrlPressed  bool
	ShiftPressed bool
}

func (CmdCastRequest) commandMarker() {}

// CmdDispel — player asked to cancel one of their active buffs (RequestDispel,
// ctrl/right-click on a buff icon).
type CmdDispel struct {
	CasterCharID int32
	SkillID      int32
}

func (CmdDispel) commandMarker() {}

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

// CmdChatMessage — a player sent a chat line that requires world-aware routing
// (nearby broadcast for ALL/SHOUT, name lookup for TELL). The client handler has
// already validated the text/type and echo-safe delivery is done on the loop
// goroutine so it never races the visibility/broadcast machinery.
type CmdChatMessage struct {
	SenderCharID  int32
	SenderAccount string
	ChatType      int32
	SenderName    string
	Text          string
	Target        string // only for TELL
}

func (CmdChatMessage) commandMarker() {}

// CmdRestoreStats — restore a live player's vital stats (HP/MP/CP), clamped to
// their maxima, and broadcast the resulting HP/MP/CP bars. Sent by the interim
// potion item handler (l2go-diu); the amounts are pre-resolved from the item's
// linked restore skill. A dead player (CurrentHP<=0) is left untouched.
type CmdRestoreStats struct {
	CharID int32
	HP     int32
	MP     int32
	CP     int32
	// SkillID/SkillLevel: the item's linked skill, broadcast as a MagicSkillUse
	// cast visual so the client plays the animation and starts the item icon
	// reuse-cooldown sweep. 0 = no cast visual.
	SkillID    int32
	SkillLevel int32
}

func (CmdRestoreStats) commandMarker() {}

// CmdRevive — resurrect a dead player and teleport it to a respawn point (Dest).
// Restores HP, broadcasts Revive, then teleports. Used by RequestRestartPoint.
type CmdRevive struct {
	CharID  int32
	Dest    models.Position
	Heading int32
}

func (CmdRevive) commandMarker() {}
