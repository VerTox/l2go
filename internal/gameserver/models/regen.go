package models

import "math"

// Regen amounts are applied once per RegenTick (see gameloop). Values are HP/MP/CP
// restored per tick for a living character.
//
// L2J stores per-level base regen in each class's baseStats lvlUpgainData table.
// That table is (for the classes checked) class-independent and, at retail, is the
// actual displayed regen — CON/MEN and posture (sitting/standing/running) are minor
// or absent modifiers on the *rate*. These formulas fit that shared table: exact at
// level 1 and at level >= 20 (verified against HumanFighter/HumanMystic 20/40/60/80/85),
// within ~0.1-0.5 in between. The exact table lookup plus posture/combat modifiers is
// a follow-up (l2go-y93); this level-only approximation is enough to make MP-consuming
// skills viable.

// regenCapLevel is where MP and CP regen plateau in the retail table (HP keeps
// growing to the level cap).
const regenCapLevel = 80

// HpRegenPerTick returns the HP restored per regen tick at the given level.
func HpRegenPerTick(level int) float64 {
	return math.Max(2.0, 1.4+0.1*float64(level))
}

// MpRegenPerTick returns the MP restored per regen tick at the given level.
func MpRegenPerTick(level int) float64 {
	return math.Max(0.9, 0.6+0.03*float64(minInt(level, regenCapLevel)))
}

// CpRegenPerTick returns the CP restored per regen tick at the given level.
func CpRegenPerTick(level int) float64 {
	return math.Max(2.0, 0.5+0.1*float64(minInt(level, regenCapLevel)))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
