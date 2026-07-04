package gameloop

import (
	"context"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
)

// teleportZOffset nudges the destination up a little, matching L2J (z += 5) to avoid
// dropping the character into the ground on arrival.
const teleportZOffset = 5

// handleTeleport relocates a player to a new position, mirroring L2J teleToLocation:
// abort the current action, tell the client (and everyone watching) to teleport, decay
// the player from its old surroundings, then move it and flag it teleporting. Visibility
// at the destination is re-established when the client answers with Appearing (0x3a);
// the loop does NOT touch KnownNPCs here — that set belongs to the connection goroutine.
// (l2go-3xh.2)
func (gl *GameLoop) handleTeleport(cmd CmdTeleport) {
	player, exists := gl.world.GetPlayer(cmd.CharID)
	if !exists {
		return
	}

	// Abort whatever the player was doing (L2J: stopMove, abortAttack, setTarget(null)).
	gl.stopAttacker(cmd.CharID)
	gl.world.SetPlayerTarget(cmd.CharID, 0) // clears the reverse targeter index too (l2go-45b)
	player.IsMoving = false
	player.MoveStartPos = models.Position{}
	player.MoveDestination = models.Position{}

	dest := models.Position{X: cmd.Dest.X, Y: cmd.Dest.Y, Z: cmd.Dest.Z + teleportZOffset}
	oldPos := player.Position

	// Tell the teleporting client (and everyone who sees it) to move it. Broadcast at the
	// OLD position — that's who currently has the player in view.
	telePkt := outclient.BuildTeleportToLocation(cmd.CharID,
		int32(dest.X), int32(dest.Y), int32(dest.Z), cmd.Heading)
	gl.broadcastToNearby(oldPos, telePkt)

	// Decay: drop the player from everyone else's view (the client unloads the old area
	// itself on TeleportToLocation, so exclude the player's own connection). Also clear
	// the known-set both ways so the player is spawned fresh at the destination. (l2go-23g)
	deletePkt := outclient.BuildDeleteObject(cmd.CharID)
	for _, p := range gl.world.GetPlayersInRange(oldPos, broadcastRadius) {
		if p.CharID == cmd.CharID {
			continue
		}
		if conn := gl.connections.GetConnection(p.AccountName); conn != nil {
			_ = conn.Send(deletePkt)
		}
		delete(p.KnownPlayers, cmd.CharID)
	}
	// The teleporting client unloaded its whole area, so it no longer knows anyone;
	// visibility is rebuilt on Appearing (CmdPlayerEnteredWorld → reconcile).
	player.KnownPlayers = make(map[int32]bool)

	player.IsTeleporting = true
	_ = gl.world.UpdatePlayerPosition(context.Background(), cmd.CharID, dest, cmd.Heading)
}

// handleRevive resurrects a dead player and teleports it to the respawn point. Restores
// HP to full, broadcasts Revive (clears the death state client-side), then reuses the
// teleport primitive to move the player to the destination. (l2go-3xh.4)
func (gl *GameLoop) handleRevive(cmd CmdRevive) {
	player, exists := gl.world.GetPlayer(cmd.CharID)
	if !exists || player.Character == nil {
		return
	}

	// Restore HP before Revive so the UserInfo the client gets on Appearing shows it.
	player.Character.CurrentHP = float64(player.Character.MaxHP)

	// Revive clears the death state on the reviving client and everyone watching.
	gl.broadcastToNearby(player.Position, outclient.BuildRevive(cmd.CharID))

	// Relocate to the respawn point (broadcasts TeleportToLocation, decays, moves).
	gl.handleTeleport(CmdTeleport{CharID: cmd.CharID, Dest: cmd.Dest, Heading: cmd.Heading})
}
