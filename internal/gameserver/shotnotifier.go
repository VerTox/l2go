package gameserver

import (
	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// shotEffectNotifier implements usecase.ShotEffectNotifier by sending packets to
// the shot owner and broadcasting the activation animation to nearby players.
// It bridges the domain shot handlers (which know nothing about the network) to
// the world/connection registries.
type shotEffectNotifier struct {
	world       *registry.WorldRegistry
	connections *registry.ConnectionRegistry
}

func newShotEffectNotifier(world *registry.WorldRegistry, connections *registry.ConnectionRegistry) *shotEffectNotifier {
	return &shotEffectNotifier{world: world, connections: connections}
}

// sendToOwner locates the owner's live connection and sends a packet to it.
func (n *shotEffectNotifier) sendToOwner(charID int32, packet []byte) {
	player, ok := n.world.GetPlayer(charID)
	if !ok {
		return
	}
	conn := n.connections.GetConnection(player.AccountName)
	if conn == nil {
		return
	}
	if err := conn.Send(packet); err != nil {
		log.Warn().Err(err).Int32("char_id", charID).Msg("failed to send shot packet to owner")
	}
}

func (n *shotEffectNotifier) ItemSystemMessage(charID int32, msgID int32, itemID int32) {
	n.sendToOwner(charID, outclient.NewSystemMessage(msgID).AddItemName(itemID).Build())
}

func (n *shotEffectNotifier) SystemMessage(charID int32, msgID int32) {
	n.sendToOwner(charID, outclient.BuildSystemMessageNoParams(msgID))
}

// SystemMessageWithInt sends a system message carrying a single int parameter
// (e.g. UP_TO_S1_RECIPES_CAN_REGISTER with the recipe limit). This lets the same
// notifier satisfy usecase.RecipeNotifier alongside ShotEffectNotifier.
func (n *shotEffectNotifier) SystemMessageWithInt(charID int32, msgID int32, value int32) {
	n.sendToOwner(charID, outclient.NewSystemMessage(msgID).AddInt(value).Build())
}

// BroadcastShotVisual broadcasts a MagicSkillUse animation for the shot to the
// owner and every player within visibility range.
func (n *shotEffectNotifier) BroadcastShotVisual(charID int32, skillID int32, skillLevel int32) {
	player, ok := n.world.GetPlayer(charID)
	if !ok {
		return
	}

	// Player object id == char id in this server. Self-cast: caster == target.
	pkt := outclient.BuildMagicSkillUse(
		charID, charID, skillID, skillLevel, 0, 0,
		int32(player.Position.X), int32(player.Position.Y), int32(player.Position.Z),
	)

	for _, nearby := range n.world.GetPlayersInRange(player.Position, registry.VisibilityWatchRadius) {
		conn := n.connections.GetConnection(nearby.AccountName)
		if conn == nil {
			continue
		}
		if err := conn.Send(pkt); err != nil {
			log.Warn().Err(err).Int32("char_id", nearby.CharID).Msg("failed to broadcast shot visual")
		}
	}
}
