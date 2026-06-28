package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// SkillList packet (opcode 0x5f) - sends character skills
type SkillList struct {
	Skills []SkillInfo `json:"skills"`
}

// SkillInfo represents a skill in the skill list
type SkillInfo struct {
	SkillID       int32 `json:"skill_id"`
	SkillLevel    int32 `json:"skill_level"`
	IsPassive     bool  `json:"is_passive"`
	IsDisabled    bool  `json:"is_disabled"`
	IsEnchantable bool  `json:"is_enchantable"`
}

// BuildSkillList creates SkillList packet data
func BuildSkillList(skillList SkillList) []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x5f) // SkillList opcode

	// Skill count
	b.WriteD(int32(len(skillList.Skills)))

	// Write each skill
	for _, skill := range skillList.Skills {
		b.WriteD(int32(skill.SkillID))
		b.WriteD(int32(skill.SkillLevel))

		// Passive flag
		if skill.IsPassive {
			b.WriteD(0x01)
		} else {
			b.WriteD(0x00)
		}

		// Disabled flag
		if skill.IsDisabled {
			b.WriteD(0x01)
		} else {
			b.WriteD(0x00)
		}

		// Enchantable flag
		if skill.IsEnchantable {
			b.WriteD(0x01)
		} else {
			b.WriteD(0x00)
		}
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
		{SkillID: 1177, SkillLevel: 1, IsPassive: false, IsDisabled: false, IsEnchantable: false}, // Wind Strike
	}

	return NewSkillList(skills)
}
