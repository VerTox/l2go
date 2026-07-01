package gameloop

import (
	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
)

const (
	// chatAllRadius is the range for local (ALL) chat — L2J/HF ChatNormalRange (1250).
	chatAllRadius = 1250
	// chatShoutRadius is the range for SHOUT (!) chat — L2J/HF ChatShoutRange (7000).
	// L2J default is regional (MapRegion vicinity); we have no map-region graph yet,
	// so we approximate the region with this wide radius. (NOTES)
	chatShoutRadius = 7000
)

// handleChatMessage routes a validated chat line. Runs on the game-loop goroutine
// so nearby broadcast / echo delivery never races player visibility.
//
// Only channels backed by an implemented system are handled here:
//   - ALL   → CreatureSay to players within chatAllRadius (the sender is in range,
//     so the echo to self is included — matches L2J's explicit self-send).
//   - SHOUT → CreatureSay to players within chatShoutRadius (region approximation).
//   - TELL  → CreatureSay to the named online player + an echo to the sender whose
//     speaker name is "->Target" (L2J TypeTell). Offline target → SystemMessage.
//
// Other channels (PARTY/CLAN/ALLIANCE/TRADE/MPCC/…) depend on systems that do not
// exist yet and are dropped with a debug log by the client handler before reaching
// the loop.
func (gl *GameLoop) handleChatMessage(cmd CmdChatMessage) {
	pkt := outclient.BuildCreatureSay(cmd.SenderCharID, cmd.ChatType, cmd.SenderName, cmd.Text)

	switch cmd.ChatType {
	case outclient.ChatAll:
		sender, ok := gl.world.GetPlayer(cmd.SenderCharID)
		if !ok {
			return
		}
		gl.broadcastToNearbyRadius(sender.Position, pkt, chatAllRadius)

	case outclient.ChatShout:
		sender, ok := gl.world.GetPlayer(cmd.SenderCharID)
		if !ok {
			return
		}
		gl.broadcastToNearbyRadius(sender.Position, pkt, chatShoutRadius)

	case outclient.ChatTell:
		target, ok := gl.world.GetPlayerByName(cmd.Target)
		if !ok {
			// Target not online — notify the sender (L2J TARGET_IS_NOT_FOUND_IN_THE_GAME).
			if conn := gl.connections.GetConnection(cmd.SenderAccount); conn != nil {
				_ = conn.Send(outclient.BuildSystemMessageNoParams(outclient.SysMsgTargetNotFound))
			}
			return
		}
		// Deliver to the recipient.
		gl.sendToPlayer(target, pkt)
		// Echo to the sender with the "->Target" speaker label (retail behaviour).
		if conn := gl.connections.GetConnection(cmd.SenderAccount); conn != nil {
			echo := outclient.BuildCreatureSay(cmd.SenderCharID, cmd.ChatType, "->"+target.Character.Name, cmd.Text)
			_ = conn.Send(echo)
		}

	default:
		log.Debug().Int32("type", cmd.ChatType).Msg("chat channel not implemented")
	}
}

// broadcastToNearbyRadius sends packet data to all players within the given radius
// of pos (inclusive of a player standing at pos, i.e. the sender). Unlike
// broadcastToNearby it takes an explicit radius so chat channels can use their own
// ranges (local vs shout) instead of the movement/visibility broadcast radius.
func (gl *GameLoop) broadcastToNearbyRadius(pos models.Position, data []byte, radius int) {
	for _, p := range gl.world.GetPlayersInRange(pos, radius) {
		if conn := gl.connections.GetConnection(p.AccountName); conn != nil {
			_ = conn.Send(data)
		}
	}
}
