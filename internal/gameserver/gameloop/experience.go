package gameloop

import (
	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/data"
	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
	"github.com/VerTox/l2go/internal/gameserver/usecase"
)

// awardExpForNPCKill distributes EXP and SP to all attackers from the hate list
// proportionally to their damage dealt.
func (gl *GameLoop) awardExpForNPCKill(npc *models.NpcInstance) {
	hl, ok := gl.npcHateLists[npc.ObjectID]
	if !ok || hl.IsEmpty() {
		return
	}

	npcLevel := 1
	if npc.Template != nil {
		npcLevel = npc.Template.Level
	}

	baseExp := data.CalcNPCBaseExp(npcLevel)
	baseSP := data.CalcNPCBaseSP(npcLevel)

	// Compute total hate for proportional distribution
	var totalHate int64
	attackers := hl.GetAllAttackers()
	hateValues := make(map[int32]int64, len(attackers))
	for _, charID := range attackers {
		h := hl.entries[charID]
		hateValues[charID] = h
		totalHate += h
	}
	if totalHate <= 0 {
		totalHate = 1
	}

	for _, charID := range attackers {
		player, exists := gl.world.GetPlayer(charID)
		if !exists || player.Character == nil {
			continue
		}

		// Proportion based on damage dealt
		proportion := float64(hateValues[charID]) / float64(totalHate)

		// Level penalty
		penalty := data.LevelPenalty(player.Character.Level, npcLevel)

		// Final EXP/SP with server rates
		earnedExp := int64(float64(baseExp) * proportion * penalty * gl.expRate)
		earnedSP := int64(float64(baseSP) * proportion * penalty * gl.spRate)

		if earnedExp < 1 {
			earnedExp = 1
		}
		if earnedSP < 0 {
			earnedSP = 0
		}

		// Apply EXP and SP
		oldLevel := player.Character.Level
		player.Character.Experience += earnedExp
		player.Character.SP += int(earnedSP)

		// Check level-up
		newLevel := data.LevelForExp(player.Character.Experience)
		if newLevel > data.MaxLevel {
			newLevel = data.MaxLevel
		}

		leveledUp := newLevel > oldLevel

		if leveledUp {
			player.Character.Level = newLevel
			gl.applyLevelUp(player, oldLevel, newLevel)
			// Persist immediately: a level-up is the most painful progress to lose
			// on a crash. Async (value-copy) write, so it does not stall the loop.
			gl.persistPlayer(player)
		}

		// Send notifications to the player
		gl.sendExpRewardNotification(player, earnedExp, int32(earnedSP), leveledUp)

		log.Debug().
			Int32("char_id", charID).
			Int64("exp", earnedExp).
			Int64("sp", earnedSP).
			Int("level", player.Character.Level).
			Bool("leveled_up", leveledUp).
			Msg("EXP/SP awarded")
	}
}

// applyLevelUp recalculates stats and restores HP/MP on level up.
func (gl *GameLoop) applyLevelUp(player *registry.PlayerWorldState, oldLevel, newLevel int) {
	char := player.Character

	// Recompute stats for new level
	baseStats := models.CharacterStats{
		STR: char.BaseSTR,
		DEX: char.BaseDEX,
		CON: char.BaseCON,
		INT: char.BaseINT,
		WIT: char.BaseWIT,
		MEN: char.BaseMEN,
	}
	combat := usecase.GetCombatBaseStatsByClass(char.ClassID)
	computed := models.ComputeStats(baseStats, newLevel, combat)

	// Update max HP/MP (use computed values scaled by CON/MEN bonuses)
	// Use template-based scaling: maxHP grows ~10% per level as rough approximation
	hpGrowth := 1.0 + float64(newLevel-oldLevel)*0.10
	newMaxHP := int(float64(char.MaxHP) * hpGrowth)
	if computed.MaxHP > 0 {
		newMaxHP = computed.MaxHP
	}
	if newMaxHP < 1 {
		newMaxHP = 1
	}

	newMaxMP := char.MaxMP
	if computed.MaxMP > 0 {
		newMaxMP = computed.MaxMP
	}

	char.MaxHP = newMaxHP
	char.MaxMP = newMaxMP

	// Restore HP/MP to full on level up
	char.CurrentHP = float64(newMaxHP)
	char.CurrentMP = float64(newMaxMP)
}

// sendExpRewardNotification sends SystemMessage and UserInfo to a player after earning EXP.
// Uses UserInfo instead of StatusUpdate for EXP/Level/SP because StatusUpdate uses 32-bit values
// which truncate EXP at higher levels. UserInfo uses WriteQ (64-bit) for EXP.
func (gl *GameLoop) sendExpRewardNotification(player *registry.PlayerWorldState, earnedExp int64, earnedSP int32, leveledUp bool) {
	conn := gl.connections.GetConnection(player.AccountName)
	if conn == nil {
		return
	}

	// Send "You earned X experience and Y SP" SystemMessage
	if earnedSP > 0 {
		msg := outclient.NewSystemMessage(outclient.SysMsgEarnedS1ExpAndS2SP).
			AddLong(earnedExp).
			AddInt(earnedSP).
			Build()
		_ = conn.Send(msg)
	} else {
		msg := outclient.NewSystemMessage(outclient.SysMsgEarnedS1Exp).
			AddLong(earnedExp).
			Build()
		_ = conn.Send(msg)
	}

	// Send "Your level has increased!" if leveled up
	if leveledUp {
		msg := outclient.BuildSystemMessageNoParams(outclient.SysMsgYouIncreasedYourLevel)
		_ = conn.Send(msg)
	}

	// Send UserInfo with full 64-bit EXP, Level, SP, HP, MP
	userInfoData := gl.buildUserInfoForPlayer(player)
	_ = conn.Send(userInfoData)

	// Also send StatusUpdate for HP/MP bars (these are 32-bit safe)
	hpMpAttrs := []outclient.StatusAttribute{
		{ID: outclient.StatusCurHP, Value: int32(player.Character.CurrentHP)},
		{ID: outclient.StatusMaxHP, Value: int32(player.Character.MaxHP)},
		{ID: outclient.StatusCurMP, Value: int32(player.Character.CurrentMP)},
		{ID: outclient.StatusMaxMP, Value: int32(player.Character.MaxMP)},
	}
	su := outclient.BuildStatusUpdate(player.CharID, hpMpAttrs)
	_ = conn.Send(su)
}

// buildUserInfoForPlayer creates a UserInfo packet from player world state.
// Used by the game loop to send updated EXP/Level/SP without handler dependencies.
func (gl *GameLoop) buildUserInfoForPlayer(player *registry.PlayerWorldState) []byte {
	char := player.Character

	// Compute derived combat stats
	computed := gl.computePlayerStats(player)

	// Compute EXP percent towards next level
	expPercent := data.ExpPercent(char.Level, char.Experience)

	var runningFlag int32 = 1
	if !player.IsRunning {
		runningFlag = 0
	}

	var inCombatFlag int32 = 0
	if player.InCombat {
		inCombatFlag = 1
	}

	// Папердолл из кэшированных на персонаже слотов (display + object IDs), чтобы
	// UserInfo из game loop (старт боя, combat stance, level up) не «раздевал» персонажа.
	paperdoll := outclient.NewPaperdollInfo()
	for slot := 0; slot < len(char.PaperdollItems); slot++ {
		paperdoll.DisplayIDs[slot] = char.PaperdollItems[slot]
		paperdoll.ObjectIDs[slot] = char.PaperdollObjectIDs[slot]
	}

	userInfo := outclient.UserInfo{
		X:        int32(player.Position.X),
		Y:        int32(player.Position.Y),
		Z:        int32(player.Position.Z),
		ObjectID: char.ID,
		Name:     char.Name,
		Race:     int32(char.Race),
		Sex:      int32(char.Sex),
		ClassID:  int32(char.ClassID),
		Level:    int32(char.Level),
		EXP:      char.Experience,
		STR:      int32(char.BaseSTR),
		DEX:      int32(char.BaseDEX),
		CON:      int32(char.BaseCON),
		INT:      int32(char.BaseINT),
		WIT:      int32(char.BaseWIT),
		MEN:      int32(char.BaseMEN),
		MaxHP:     int32(char.MaxHP),
		CurrentHP: int32(char.CurrentHP),
		MaxMP:     int32(char.MaxMP),
		CurrentMP: int32(char.CurrentMP),
		MaxCP:     int32(char.MaxCP),
		CurrentCP: int32(char.CurrentCP),
		CurrentSP:   int64(char.SP),
		CurrentLoad: 0,
		MaxLoad:     int32(computed.MaxLoad),
		PAtk:     int32(computed.PAtk),
		AtkSpd:   int32(computed.PAtkSpd),
		PDef:     int32(computed.PDef),
		Evasion:  int32(computed.Evasion),
		Accuracy: int32(computed.Accuracy),
		Critical: int32(computed.CritRate),
		MAtk:     int32(computed.MAtk),
		CastSpd:  int32(computed.MAtkSpd),
		MDef:     int32(computed.MDef),
		PvPFlag:    0,
		Karma:      int32(char.Karma),
		RunSpd:     int32(computed.RunSpd),
		WalkSpd:    int32(computed.WalkSpd),
		SwimRunSpd: int32(computed.SwimRunSpd),
		SwimWalkSpd: int32(computed.SwimWalkSpd),
		ClanID:     int32(char.ClanID),
		PKKills:    int32(char.PKKills),
		PVPKills:   int32(char.PvPKills),
		Cubics:     []int32{},
		ClassId2:   int32(char.ClassID),
		InventoryLimit: 80,
		RunningFlag: runningFlag,
		InCombat:    inCombatFlag,
		ExpPercent:  expPercent / 100.0, // UserInfo expects 0.0-1.0 fraction
		HairStyle:   int32(char.HairStyle),
		HairColor:   int32(char.HairColor),
		Face:        int32(char.Face),
		MinimapAllowed: 1,
		CanEquipCloak:  1,
		// Collision values based on race/sex
		CollisionRadius: getCollisionRadiusForPlayer(char.Race, char.Sex),
		CollisionHeight: getCollisionHeightForPlayer(char.Race, char.Sex),
		// Экипировка — иначе клиент показывает персонажа «голым» при боевом UserInfo.
		Paperdoll: paperdoll,
	}

	return outclient.BuildUserInfo(userInfo)
}

// Collision lookup tables (duplicated from handlers/client/world.go to avoid circular dependency)
type playerCollisionKey struct{ race, sex int }

var playerCollisionRadiusMap = map[playerCollisionKey]float64{
	{0, 0}: 9.0, {0, 1}: 8.0, {1, 0}: 7.5, {1, 1}: 7.5,
	{2, 0}: 7.5, {2, 1}: 7.0, {3, 0}: 11.0, {3, 1}: 7.0,
	{4, 0}: 9.0, {4, 1}: 5.0, {5, 0}: 8.0, {5, 1}: 7.0,
}
var playerCollisionHeightMap = map[playerCollisionKey]float64{
	{0, 0}: 23.0, {0, 1}: 23.5, {1, 0}: 24.0, {1, 1}: 23.0,
	{2, 0}: 24.0, {2, 1}: 23.5, {3, 0}: 28.0, {3, 1}: 27.0,
	{4, 0}: 18.0, {4, 1}: 19.0, {5, 0}: 25.2, {5, 1}: 22.6,
}

func getCollisionRadiusForPlayer(race, sex int) float64 {
	if r, ok := playerCollisionRadiusMap[playerCollisionKey{race, sex}]; ok {
		return r
	}
	return 9.0
}

func getCollisionHeightForPlayer(race, sex int) float64 {
	if h, ok := playerCollisionHeightMap[playerCollisionKey{race, sex}]; ok {
		return h
	}
	return 23.0
}
