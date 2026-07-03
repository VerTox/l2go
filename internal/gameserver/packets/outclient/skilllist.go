package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// SkillList packet (opcode 0x5f) - sends character skills
type SkillList struct {
	Skills []SkillInfo `json:"skills"`
}

// SkillInfo represents a skill in the skill list
type SkillInfo struct {
	SkillID     int32 `json:"skill_id"`
	SkillLevel  int32 `json:"skill_level"`
	IsPassive   bool  `json:"is_passive"`
	IsDisabled  bool  `json:"is_disabled"`
	IsEnchanted bool  `json:"is_enchanted"` // skill level > 100 (enchant route)
}

// BuildSkillList creates SkillList packet data.
//
// Byte layout mirrors L2J High Five SkillList.writeImpl exactly:
//
//	C  0x5F
//	D  skill count
//	per skill:
//	  D  passive (1/0)
//	  D  level
//	  D  id
//	  C  disabled (1/0)
//	  C  enchanted (1/0)
func BuildSkillList(skillList SkillList) []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x5f) // SkillList opcode
	b.WriteD(int32(len(skillList.Skills)))

	for _, skill := range skillList.Skills {
		b.WriteD(boolToD(skill.IsPassive))
		b.WriteD(skill.SkillLevel)
		b.WriteD(skill.SkillID)
		b.WriteC(boolToC(skill.IsDisabled))
		b.WriteC(boolToC(skill.IsEnchanted))
	}

	return b.Bytes()
}

// NewSkillList creates a skill list with basic skills
func NewSkillList(skills []SkillInfo) []byte {
	skillList := SkillList{
		Skills: skills,
	}

	return BuildSkillList(skillList)
}

// NewEmptySkillList creates an empty skill list
func NewEmptySkillList() []byte {
	skillList := SkillList{
		Skills: []SkillInfo{},
	}

	return BuildSkillList(skillList)
}

// NewBasicSkillList creates a skill list with basic starting skills
func NewBasicSkillList() []byte {
	skills := []SkillInfo{
		{SkillID: 1177, SkillLevel: 1, IsPassive: false, IsDisabled: false, IsEnchanted: false}, // Wind Strike
	}

	return NewSkillList(skills)
}
