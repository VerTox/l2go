package models

import "math"

// CombatBaseStats holds the per-class combat base values from pcBaseStats.xml
type CombatBaseStats struct {
	BasePAtk    int
	BaseMAtk    int
	BaseCritRate int
	BasePAtkSpd int
	BaseMAtkSpd int

	// Per-slot physical defense
	BasePDefChest     int
	BasePDefLegs      int
	BasePDefHead      int
	BasePDefFeet      int
	BasePDefGloves    int
	BasePDefUnderwear int
	BasePDefCloak     int

	// Per-slot magical defense
	BaseMDefREar    int
	BaseMDefLEar    int
	BaseMDefRFinger int
	BaseMDefLFinger int
	BaseMDefNeck    int

	// Movement speeds
	BaseRunSpd     int
	BaseWalkSpd    int
	BaseSwimRunSpd int
	BaseSwimWalkSpd int
}

// TotalBasePDef returns the sum of all armor slot base defenses
func (c *CombatBaseStats) TotalBasePDef() int {
	return c.BasePDefChest + c.BasePDefLegs + c.BasePDefHead +
		c.BasePDefFeet + c.BasePDefGloves + c.BasePDefUnderwear + c.BasePDefCloak
}

// TotalBaseMDef returns the sum of all jewelry slot base defenses
func (c *CombatBaseStats) TotalBaseMDef() int {
	return c.BaseMDefREar + c.BaseMDefLEar + c.BaseMDefRFinger +
		c.BaseMDefLFinger + c.BaseMDefNeck
}

// ComputedStats holds all derived combat stats for a character
type ComputedStats struct {
	PAtk     int
	MAtk     int
	PDef     int
	MDef     int
	Accuracy int
	Evasion  int
	CritRate int
	PAtkSpd  int
	MAtkSpd  int
	RunSpd   int
	WalkSpd  int
	SwimRunSpd  int
	SwimWalkSpd int
	MaxHP    int
	MaxMP    int
	MaxCP    int
	MaxLoad  int
}

// ComputeStats calculates all derived combat stats from base stats, level, and combat base stats.
// This implements the L2J stat calculation formulas.
func ComputeStats(baseStats CharacterStats, level int, combat CombatBaseStats) ComputedStats {
	levelMod := LevelMod(level)
	strB := STRBonus(baseStats.STR)
	intB := INTBonus(baseStats.INT)
	dexB := DEXBonus(baseStats.DEX)
	witB := WITBonus(baseStats.WIT)
	conB := CONBonus(baseStats.CON)
	menB := MENBonus(baseStats.MEN)

	// PAtk = basePAtk * STR_bonus * LevelMod
	patk := float64(combat.BasePAtk) * strB * levelMod

	// MAtk = baseMAtk * INT_bonus^2 * LevelMod^2
	matk := float64(combat.BaseMAtk) * intB * intB * levelMod * levelMod

	// PDef = totalBasePDef * LevelMod (naked character — all slots have base defense)
	pdef := float64(combat.TotalBasePDef()) * levelMod

	// MDef = totalBaseMDef * MEN_bonus * LevelMod
	mdef := float64(combat.TotalBaseMDef()) * menB * levelMod

	// Accuracy = sqrt(DEX)*6 + Level + level_bonus_thresholds
	accuracy := CalcAccuracy(baseStats.DEX, level)

	// Evasion = sqrt(DEX)*6 + Level + level_bonus_thresholds
	evasion := CalcEvasion(baseStats.DEX, level)

	// CritRate = baseCritRate * DEX_bonus * 10
	critRate := float64(combat.BaseCritRate) * dexB * 10.0

	// PAtkSpd = basePAtkSpd * DEX_bonus
	patkSpd := float64(combat.BasePAtkSpd) * dexB

	// MAtkSpd = baseMAtkSpd * WIT_bonus
	matkSpd := float64(combat.BaseMAtkSpd) * witB

	// RunSpd = baseRunSpd * DEX_bonus
	runSpd := float64(combat.BaseRunSpd) * dexB

	// WalkSpd = baseWalkSpd * DEX_bonus
	walkSpd := float64(combat.BaseWalkSpd) * dexB

	// SwimRunSpd
	swimRunSpd := float64(combat.BaseSwimRunSpd) * dexB

	// SwimWalkSpd
	swimWalkSpd := float64(combat.BaseSwimWalkSpd) * dexB

	// MaxHP = baseHP * CON_bonus (baseHP from template, stored in Character.MaxHP at creation)
	// MaxMP = baseMP * MEN_bonus
	// MaxCP = baseCP * CON_bonus
	// Note: these use the template HP/MP/CP values which are already per-level for level 1
	// For now we don't have per-level arrays, so we skip re-calculating MaxHP/MP/CP here
	// and let the caller pass them through. The caller can multiply template values by bonuses.

	// Weight limit = 69000 * CON_bonus
	maxLoad := int(69000.0 * conB)

	return ComputedStats{
		PAtk:        int(math.Round(patk)),
		MAtk:        int(math.Round(matk)),
		PDef:        int(math.Round(pdef)),
		MDef:        int(math.Round(mdef)),
		Accuracy:    int(math.Round(accuracy)),
		Evasion:     int(math.Round(evasion)),
		CritRate:    int(math.Round(critRate)),
		PAtkSpd:     int(math.Round(patkSpd)),
		MAtkSpd:     int(math.Round(matkSpd)),
		RunSpd:      int(math.Round(runSpd)),
		WalkSpd:     int(math.Round(walkSpd)),
		SwimRunSpd:  int(math.Round(swimRunSpd)),
		SwimWalkSpd: int(math.Round(swimWalkSpd)),
		MaxLoad:     maxLoad,
	}
}

// ComputeMaxHP calculates max HP from template base HP and CON
func ComputeMaxHP(baseHP int, con int) int {
	return int(math.Round(float64(baseHP) * CONBonus(con)))
}

// ComputeMaxMP calculates max MP from template base MP and MEN
func ComputeMaxMP(baseMP int, men int) int {
	return int(math.Round(float64(baseMP) * MENBonus(men)))
}

// ComputeMaxCP calculates max CP from template base CP and CON
func ComputeMaxCP(baseCP int, con int) int {
	return int(math.Round(float64(baseCP) * CONBonus(con)))
}
