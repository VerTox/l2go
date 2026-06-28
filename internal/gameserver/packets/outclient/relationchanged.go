package outclient

import (
	"fmt"
	"github.com/VerTox/l2go/pkg/l2pkt"
)

// RelationChanged packet (0xCE) - controls player targetability and cursor display
// This packet is crucial for preventing sword cursor on non-hostile players
type RelationChanged struct {
	Relations []PlayerRelation
}

// PlayerRelation represents relation data for one player
type PlayerRelation struct {
	ObjectID       int32 // Player Object ID
	Relation       int32 // Relation flags (0 = normal, non-hostile)
	AutoAttackable int32 // 0 = not attackable, 1 = attackable
	Karma          int32 // Player's karma value
	PvPFlag        int32 // Player's PvP flag
}

// Relation flag constants from Java L2J
const (
	RelationNone         = 0x00000 // Normal player (no sword cursor)
	RelationParty1       = 0x00001
	RelationParty2       = 0x00002
	RelationParty3       = 0x00004
	RelationParty4       = 0x00008
	RelationPartyLeader  = 0x00010
	RelationHasParty     = 0x00020
	RelationClanMember   = 0x00040
	RelationLeader       = 0x00080
	RelationClanMate     = 0x00100
	RelationInSiege      = 0x00200
	RelationAttacker     = 0x00400
	RelationAlly         = 0x00800
	RelationEnemy        = 0x01000
	RelationMutualWar    = 0x04000
	RelationOneSidedWar  = 0x08000
	RelationAllyMember   = 0x10000
	RelationTerritoryWar = 0x80000
)

// NewRelationChanged creates a new RelationChanged packet
func NewRelationChanged(relations []PlayerRelation) *RelationChanged {
	return &RelationChanged{
		Relations: relations,
	}
}

// NewSingleRelation creates RelationChanged packet for single player (normal non-hostile)
func NewSingleRelation(objectID, karma, pvpFlag int32) *RelationChanged {
	relation := PlayerRelation{
		ObjectID:       objectID,
		Relation:       RelationNone, // Normal player - no special relation
		AutoAttackable: 0,            // Not attackable - prevents sword cursor
		Karma:          karma,
		PvPFlag:        pvpFlag,
	}
	
	return &RelationChanged{
		Relations: []PlayerRelation{relation},
	}
}

// BuildRelationChanged creates RelationChanged packet data using pkg/l2pkt
func BuildRelationChanged(relations []PlayerRelation) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0xCE) // RelationChanged opcode
	
	// Write count of relations
	w.WriteD(int32(len(relations)))
	
	// Write each relation
	for _, rel := range relations {
		w.WriteD(rel.ObjectID)       // Player Object ID
		w.WriteD(rel.Relation)       // Relation flags
		w.WriteD(rel.AutoAttackable) // Attackable flag
		w.WriteD(rel.Karma)          // Karma value
		w.WriteD(rel.PvPFlag)        // PvP flag
	}
	
	// Debug logging
	for _, rel := range relations {
		fmt.Printf("[DEBUG] RelationChanged for ObjectID:%d - Relation:%d AutoAttack:%d Karma:%d PvP:%d\n",
			rel.ObjectID, rel.Relation, rel.AutoAttackable, rel.Karma, rel.PvPFlag)
	}
	
	return w.Bytes()
}

// GetData returns the packet data bytes
func (p *RelationChanged) GetData() []byte {
	return BuildRelationChanged(p.Relations)
}