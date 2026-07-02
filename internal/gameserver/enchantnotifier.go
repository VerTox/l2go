package gameserver

import (
	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// enchantNotifier implements usecase.EnchantNotifier by sending the enchant
// prompt / feedback to the scroll owner's live connection. It bridges the domain
// enchant handler (which knows nothing about the network) to the world/connection
// registries, exactly like shotEffectNotifier.
type enchantNotifier struct {
	world       *registry.WorldRegistry
	connections *registry.ConnectionRegistry
}

func newEnchantNotifier(world *registry.WorldRegistry, connections *registry.ConnectionRegistry) *enchantNotifier {
	return &enchantNotifier{world: world, connections: connections}
}

func (n *enchantNotifier) sendToOwner(charID int32, packet []byte) {
	player, ok := n.world.GetPlayer(charID)
	if !ok {
		return
	}
	conn := n.connections.GetConnection(player.AccountName)
	if conn == nil {
		return
	}
	if err := conn.Send(packet); err != nil {
		log.Warn().Err(err).Int32("char_id", charID).Msg("failed to send enchant packet to owner")
	}
}

// ChooseInventoryItem opens the client's enchant target-selection window.
func (n *enchantNotifier) ChooseInventoryItem(charID int32, itemID int32) {
	n.sendToOwner(charID, outclient.BuildChooseInventoryItem(itemID))
}

// SystemMessage sends a parameterless system message to the scroll owner.
func (n *enchantNotifier) SystemMessage(charID int32, msgID int32) {
	n.sendToOwner(charID, outclient.BuildSystemMessageNoParams(msgID))
}
