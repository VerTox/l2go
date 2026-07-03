package gameloop

import "math/rand"

// calcAttackSpeed returns the time between attacks in milliseconds.
// Formula from L2J: 500000 / PAtkSpd
func calcAttackSpeed(pAtkSpd int) int {
	if pAtkSpd <= 0 {
		pAtkSpd = 1
	}
	ms := 500000 / pAtkSpd
	if ms < 100 {
		ms = 100 // minimum 100ms between attacks
	}
	return ms
}

// calcHitChance returns true if the attack hits.
// Formula: clamp(80 + 2*(accuracy - evasion), 20, 98)
func calcHitChance(accuracy, evasion int) bool {
	chance := 80 + 2*(accuracy-evasion)
	if chance < 20 {
		chance = 20
	}
	if chance > 98 {
		chance = 98
	}
	return rand.Intn(100) < chance
}

// soulshotPAtk doubles pAtk when the weapon holds a soulshot charge, mirroring
// L2J Formulas.calcPhysDam ssboost (applied to pAtk before defence/crit/variance).
func soulshotPAtk(pAtk int, charged bool) int {
	if charged {
		return pAtk * 2
	}
	return pAtk
}

// calcPhysDamage computes physical damage.
// Formula: (76 * PAtk) / PDef, minimum 1
func calcPhysDamage(pAtk, pDef int) int32 {
	if pDef < 1 {
		pDef = 1
	}
	damage := (76 * pAtk) / pDef
	if damage < 1 {
		damage = 1
	}
	return int32(damage)
}

// calcCrit returns true if the attack is a critical hit.
func calcCrit(critRate int) bool {
	// critRate is typically 1-100 (L2J: critRate/10 gives percent)
	chance := critRate
	if chance < 1 {
		chance = 1
	}
	if chance > 100 {
		chance = 100
	}
	return rand.Intn(100) < chance
}

// applyVariance applies +-10% random variance to damage.
func applyVariance(damage int32) int32 {
	if damage <= 0 {
		return damage
	}
	// variance: 0.90 .. 1.10
	variance := 0.90 + rand.Float64()*0.20
	result := int32(float64(damage) * variance)
	if result < 1 {
		result = 1
	}
	return result
}
