package gameloop

import (
	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// defaultPhysicalAttackRange is the L2J base melee range for an unarmed character
// (datapack weapon "attack_range" defaults to 40). Used until the weapon/stat
// system exists.
const defaultPhysicalAttackRange = 40

// interactApproachBase is the L2J base offset for walking up to an NPC before
// interacting (L2PlayerAI.thinkInteract → maybeMoveToPawn(target, 36)). Collision
// radii are added on top, same as melee, keeping the client well inside the wider
// INTERACTION_DISTANCE (150) dialogue check for NPCs of any size.
const interactApproachBase = 36

// collisionRadii returns the player's and the npc's collision radii, defaulting to
// 0 when data is missing.
func (gl *GameLoop) collisionRadii(player *registry.PlayerWorldState, npc *models.NpcInstance) (playerCollision, npcCollision float64) {
	if player.Character != nil {
		playerCollision = getCollisionRadiusForPlayer(player.Character.Race, player.Character.Sex)
	}
	if npc.Template != nil {
		npcCollision = npc.Template.CollisionRadius
	}
	return playerCollision, npcCollision
}

// attackReach returns the center-to-center distance within which an attack of the
// given base range connects. Per L2J (L2CharacterAI.maybeMoveToPawn) the offset is
// baseRange + attacker collision radius + target collision radius, and the very
// same value is sent as the MoveToPawn offset and used for the hit-range check.
// baseRange is the physical attack range for normal hits, or the skill cast range
// for skills — the caller chooses.
func attackReach(baseRange int, attackerCollision, targetCollision float64) int {
	return baseRange + int(attackerCollision) + int(targetCollision)
}

// playerPhysicalAttackRange returns a player's physical attack range.
// TODO: derive from the equipped weapon's attack_range and the POWER_ATTACK_RANGE
// stat (buffs / equipment bonuses), per L2J CharStat.getPhysicalAttackRange. The
// weapon/stat system does not exist yet, so we use the unarmed base.
func playerPhysicalAttackRange(_ *registry.PlayerWorldState) int {
	return defaultPhysicalAttackRange
}

// meleeReach computes the attack reach for a normal melee hit of player vs npc,
// combining the player's physical attack range with both collision radii.
func (gl *GameLoop) meleeReach(player *registry.PlayerWorldState, npc *models.NpcInstance) int {
	playerCollision, npcCollision := gl.collisionRadii(player, npc)
	return attackReach(playerPhysicalAttackRange(player), playerCollision, npcCollision)
}

// interactApproachOffset computes the MoveToPawn offset for walking up to an NPC
// to interact: the interact base plus both collision radii (mirrors meleeReach but
// with the interact base). Kept below INTERACTION_DISTANCE so the dialogue check
// passes once the client arrives.
func (gl *GameLoop) interactApproachOffset(player *registry.PlayerWorldState, npc *models.NpcInstance) int {
	playerCollision, npcCollision := gl.collisionRadii(player, npc)
	return attackReach(interactApproachBase, playerCollision, npcCollision)
}

