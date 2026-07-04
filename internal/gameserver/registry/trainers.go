package registry

// Trainer (skill coach) tables, extracted from the L2J HF datapack AI scripts
// (ai/npc/coach/{Fighter,Mage,Cleric,Wizard,Kamael}Coach). A trainer NPC teaches a
// player only if the player's race (and sex, for Kamael) + class category match one
// of the coach's conditions. Skills shown are always the player's own class tree.
// (l2go-hv9)

// coachCond is one (race, [sex,] category) teaching condition. sex == -1 = any sex.
type coachCond struct {
	race     int
	sex      int
	category string
}

type coachDef struct {
	conds []coachCond
}

var (
	fighterConds = []coachCond{
		{0, -1, "HUMAN_FALL_CLASS"},
		{1, -1, "ELF_FALL_CLASS"},
		{2, -1, "DELF_FALL_CLASS"},
		{3, -1, "ORC_FALL_CLASS"},
		{4, -1, "DWARF_SMITH_CLASS"},
	}
	mageConds = []coachCond{
		{0, -1, "HUMAN_MALL_CLASS"},
		{1, -1, "ELF_MALL_CLASS"},
		{2, -1, "DELF_MALL_CLASS"},
		{3, -1, "ORC_MALL_CLASS"},
	}
	clericConds = []coachCond{
		{0, -1, "HUMAN_CALL_CLASS"},
		{1, -1, "ELF_CALL_CLASS"},
		{2, -1, "DELF_CALL_CLASS"},
	}
	wizardConds = []coachCond{
		{0, -1, "HUMAN_WALL_CLASS"},
		{1, -1, "ELF_WALL_CLASS"},
		{2, -1, "DELF_WALL_CLASS"},
	}
	kamaelConds = []coachCond{
		{5, 0, "KAMAEL_MALE_MAIN_OCCUPATION"},
		{5, 1, "KAMAEL_FEMALE_MAIN_OCCUPATION"},
	}
)

var fighterCoachNPCs = []int32{
	30010, 30014, 30027, 30028, 30029, 30064, 30065, 30105, 30106, 30107, 30108,
	30143, 30155, 30156, 30184, 30185, 30186, 30192, 30325, 30326, 30327, 30328,
	30329, 30360, 30369, 30374, 30378, 30459, 30460, 30463, 30472, 30475, 30501,
	30506, 30509, 30514, 30569, 30570, 30679, 30683, 30690, 30691, 30692, 30693,
	30700, 30705, 30850, 30851, 30852, 30853, 30863, 30866, 30901, 30902, 30903,
	30904, 30911, 30914, 31277, 31278, 31286, 31289, 31322, 31323, 31325, 31327,
	31580, 31582, 31966, 31967, 31975, 31978, 32148, 32151, 32156, 32161,
}

var mageCoachNPCs = []int32{
	30144, 30145, 30158, 30194, 30330, 30377, 30464, 30476, 30502, 30507,
	30510, 30515, 30571, 30572, 30682, 30701, 30706, 30864, 30867, 30912,
	30915, 31287, 31290, 31335, 31337, 31581, 31976, 31979, 32152, 32162,
}

var clericCoachNPCs = []int32{
	30022, 30030, 30032, 30036, 30067, 30068, 30116, 30117, 30118, 30119,
	30188, 30293, 30375, 30473, 30680, 30858, 30859, 30860, 30861, 30906,
	30908, 31280, 31281, 31329, 31330, 31969, 31970, 32155,
}

var wizardCoachNPCs = []int32{
	30033, 30034, 30035, 30069, 30110, 30111, 30112, 30113, 30114, 30157,
	30171, 30189, 30190, 30344, 30345, 30376, 30461, 30695, 30696, 30697,
	30698, 30715, 30717, 30718, 30720, 30721, 30833, 30835, 30855, 30856,
	30907, 30909, 31282, 31283, 31332, 31333, 31971, 31972, 32149,
}

var kamaelCoachNPCs = []int32{
	32141, 32142, 32143, 32144, 32182, 32183, 32194, 32195, 32197, 32198,
	32200, 32201, 32203, 32204, 32207, 32208, 32211, 32212, 32215, 32216,
	32219, 32220, 32223, 32224, 32227, 32228, 32231, 32232,
}

// npcTrainers maps a trainer NPC id to its coach definition. Built once at init.
var npcTrainers = buildTrainers()

func buildTrainers() map[int32]*coachDef {
	m := make(map[int32]*coachDef)
	add := func(ids []int32, conds []coachCond) {
		def := &coachDef{conds: conds}
		for _, id := range ids {
			m[id] = def
		}
	}
	add(fighterCoachNPCs, fighterConds)
	add(mageCoachNPCs, mageConds)
	add(clericCoachNPCs, clericConds)
	add(wizardCoachNPCs, wizardConds)
	add(kamaelCoachNPCs, kamaelConds)
	return m
}

// IsTrainer reports whether the NPC template id is a known skill trainer.
func IsTrainer(npcID int32) bool {
	_, ok := npcTrainers[npcID]
	return ok
}

// CanTeach reports whether the trainer NPC teaches the given player (race, sex,
// classID): the NPC is a coach and one of its conditions matches the player's race
// (and sex, for Kamael) with the class in the condition's category.
func CanTeach(npcID int32, race, sex, classID int) bool {
	def, ok := npcTrainers[npcID]
	if !ok {
		return false
	}
	for _, c := range def.conds {
		if c.race != race {
			continue
		}
		if c.sex != -1 && c.sex != sex {
			continue
		}
		if GetCategoryRegistry().InCategory(c.category, classID) {
			return true
		}
	}
	return false
}
