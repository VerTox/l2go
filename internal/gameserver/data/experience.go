package data

// expTable holds cumulative EXP thresholds for each level (1–87).
// Index = level, value = total EXP required to reach that level.
// Source: L2J High Five datapack experienceData.
var expTable = [88]int64{
	0,          // 0 (unused)
	0,          // 1
	68,         // 2
	363,        // 3
	1168,       // 4
	2884,       // 5
	6038,       // 6
	11287,      // 7
	19423,      // 8
	31378,      // 9
	48229,      // 10
	71202,      // 11
	101677,     // 12
	141193,     // 13
	191454,     // 14
	254330,     // 15
	331867,     // 16
	426288,     // 17
	540000,     // 18
	675596,     // 19
	835862,     // 20
	1023784,    // 21
	1242546,    // 22
	1495543,    // 23
	1786379,    // 24
	2118876,    // 25
	2497077,    // 26
	2925250,    // 27
	3407897,    // 28
	3949754,    // 29
	4555796,    // 30
	5231246,    // 31
	5981576,    // 32
	6812513,    // 33
	7730044,    // 34
	8740422,    // 35
	9850166,    // 36
	11066072,   // 37
	12395215,   // 38
	13844951,   // 39
	15422929,   // 40
	17137087,   // 41
	18995665,   // 42
	21007203,   // 43
	23180550,   // 44
	25524868,   // 45
	28049635,   // 46
	30764654,   // 47
	33680052,   // 48
	36806289,   // 49
	40154162,   // 50
	45525133,   // 51
	51262490,   // 52
	57383988,   // 53
	63907911,   // 54
	70853089,   // 55
	80700831,   // 56
	91162654,   // 57
	102265881,  // 58
	114038596,  // 59
	126509653,  // 60
	146308200,  // 61
	167244337,  // 62
	189364894,  // 63
	212717908,  // 64
	237352644,  // 65
	271975263,  // 66
	308443198,  // 67
	346827154,  // 68
	387199547,  // 69
	429634523,  // 70
	474207979,  // 71
	532694979,  // 72
	606322775,  // 73
	696381369,  // 74
	804225364,  // 75
	931275828,  // 76
	1151275834, // 77
	1511275834, // 78
	2044287599, // 79
	3075966164, // 80
	4295351949, // 81
	5766985062, // 82
	7793077345, // 83
	10235368963, // 84
	13180481103, // 85
	16890558728, // 86
	21138534249, // 87
}

// MaxLevel is the highest reachable level.
const MaxLevel = 85

// ExpForLevel returns the total EXP needed to reach the given level.
// Returns 0 for invalid levels.
func ExpForLevel(level int) int64 {
	if level < 1 || level > 87 {
		return 0
	}
	return expTable[level]
}

// LevelForExp returns the level corresponding to total accumulated EXP.
func LevelForExp(totalExp int64) int {
	for lvl := 87; lvl >= 1; lvl-- {
		if totalExp >= expTable[lvl] {
			return lvl
		}
	}
	return 1
}

// ExpToNextLevel returns EXP remaining to reach the next level.
// Returns 0 if already at max level.
func ExpToNextLevel(currentLevel int, currentExp int64) int64 {
	if currentLevel >= MaxLevel {
		return 0
	}
	nextLevelExp := expTable[currentLevel+1]
	remaining := nextLevelExp - currentExp
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ExpPercent returns the EXP progress percentage (0.0–100.0) towards the next level.
func ExpPercent(currentLevel int, currentExp int64) float64 {
	if currentLevel >= MaxLevel || currentLevel < 1 {
		return 0.0
	}
	curThreshold := expTable[currentLevel]
	nextThreshold := expTable[currentLevel+1]
	total := nextThreshold - curThreshold
	if total <= 0 {
		return 0.0
	}
	progress := currentExp - curThreshold
	if progress < 0 {
		return 0.0
	}
	pct := float64(progress) / float64(total) * 100.0
	if pct > 100.0 {
		pct = 100.0
	}
	return pct
}

// CalcNPCBaseExp returns the base EXP reward for killing an NPC of the given level.
// Formula: npcLevel^2 (simple L2-style base; server rate applied separately).
func CalcNPCBaseExp(npcLevel int) int64 {
	return int64(npcLevel) * int64(npcLevel)
}

// CalcNPCBaseSP returns the base SP reward for killing an NPC of the given level.
// Formula: npcLevel * 2 (simple approximation; server rate applied separately).
func CalcNPCBaseSP(npcLevel int) int64 {
	return int64(npcLevel) * 2
}

// LevelPenalty returns the EXP multiplier when player level differs from NPC level.
// For level difference > 5, penalty = (5/6)^(diff-5).
// For diff <= 5, returns 1.0 (no penalty).
func LevelPenalty(playerLevel, npcLevel int) float64 {
	diff := playerLevel - npcLevel
	if diff < 0 {
		diff = -diff
	}
	if diff <= 5 {
		return 1.0
	}
	// (5/6)^(diff-5)
	penalty := 1.0
	ratio := 5.0 / 6.0
	for i := 0; i < diff-5; i++ {
		penalty *= ratio
	}
	if penalty < 0.01 {
		penalty = 0.01 // minimum 1%
	}
	return penalty
}
