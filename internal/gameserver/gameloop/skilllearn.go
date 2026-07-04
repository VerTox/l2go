package gameloop

import (
	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/packets/outclient"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// trainerInteractDistance is the L2J canInteract range for a trainer NPC.
const trainerInteractDistance = 150

// LearnedSkill is enqueued to the persist sink after a successful learn so the DB
// write happens off the game-loop goroutine.
type LearnedSkill struct {
	CharID  int32
	SkillID int32
	Level   int32
}

// SetSkillLearnSink wires the async channel that persists learned skills (l2go-hv9).
func (gl *GameLoop) SetSkillLearnSink(ch chan<- LearnedSkill) { gl.skillLearnSink = ch }

// canReachTrainer reports whether the player is a valid trainee at npcObjID: the NPC
// exists, is a trainer that teaches this player's class, and is within interact range.
func (gl *GameLoop) canReachTrainer(player *registry.PlayerWorldState, npcObjID int32) bool {
	npc, ok := gl.world.GetNPC(npcObjID)
	if !ok || npc.Template == nil {
		return false
	}
	if !registry.CanTeach(npc.TemplateID, int(player.Character.Race), int(player.Character.Sex), int(player.Character.ClassID)) {
		return false
	}
	dx := player.Position.X - npc.Position.X
	dy := player.Position.Y - npc.Position.Y
	return dx*dx+dy*dy <= trainerInteractDistance*trainerInteractDistance
}

// handleOpenSkillLearn sends the AcquireSkillList for the player's class at a trainer.
func (gl *GameLoop) handleOpenSkillLearn(cmd CmdOpenSkillLearn) {
	player, ok := gl.world.GetPlayer(cmd.CharID)
	if !ok || player.Character == nil {
		return
	}
	if !gl.canReachTrainer(player, cmd.NpcObjID) {
		return
	}
	learnable := registry.GetSkillTreeRegistry().GetLearnableSkills(
		int(player.Character.ClassID), player.Character.Level, player.KnownSkills)
	entries := make([]outclient.AcquireSkillEntry, 0, len(learnable))
	for _, s := range learnable {
		entries = append(entries, outclient.AcquireSkillEntry{
			ID: s.SkillID, Level: int32(s.Level), SP: int32(s.LevelUpSp), HasReq: false,
		})
	}
	gl.sendToPlayer(player, outclient.BuildAcquireSkillList(entries))
}

// handleSkillLearnInfo sends AcquireSkillInfo (SP cost) for one skill.
func (gl *GameLoop) handleSkillLearnInfo(cmd CmdSkillLearnInfo) {
	player, ok := gl.world.GetPlayer(cmd.CharID)
	if !ok || player.Character == nil {
		return
	}
	sl := registry.GetSkillTreeRegistry().GetSkillLearn(int(player.Character.ClassID), cmd.SkillID, int(cmd.Level))
	if sl == nil {
		return
	}
	gl.sendToPlayer(player, outclient.BuildAcquireSkillInfo(cmd.SkillID, cmd.Level, int32(sl.LevelUpSp)))
}

// handleLearnSkill validates and grants a skill: level, SP, prerequisites, trainer
// range/class. On success it deducts SP, updates the live known-skills map, enqueues
// the DB write, and refreshes the client (AcquireSkillDone + SkillList + StatusUpdate).
func (gl *GameLoop) handleLearnSkill(cmd CmdLearnSkill) {
	player, ok := gl.world.GetPlayer(cmd.CharID)
	if !ok || player.Character == nil {
		return
	}
	if !gl.canReachTrainer(player, cmd.NpcObjID) {
		return
	}
	char := player.Character
	sl := registry.GetSkillTreeRegistry().GetSkillLearn(int(char.ClassID), cmd.SkillID, int(cmd.Level))
	if sl == nil {
		return
	}
	if char.Level < sl.GetLevel {
		return
	}
	// Prerequisite skills must be known at the exact required level (L2J).
	for _, pr := range sl.PreReqs {
		if int(player.KnownSkills[pr.SkillID]) != pr.Level {
			return
		}
	}
	// Already at or above this level?
	if int(player.KnownSkills[cmd.SkillID]) >= int(cmd.Level) {
		return
	}
	if char.SP < sl.LevelUpSp {
		gl.sendToPlayer(player, outclient.BuildSystemMessageNoParams(outclient.SysMsgNotEnoughSpToLearn))
		return
	}

	char.SP -= sl.LevelUpSp
	player.KnownSkills[cmd.SkillID] = cmd.Level

	if gl.skillLearnSink != nil {
		gl.skillLearnSink <- LearnedSkill{CharID: cmd.CharID, SkillID: cmd.SkillID, Level: cmd.Level}
	}

	gl.sendToPlayer(player, outclient.BuildAcquireSkillDone())
	gl.sendToPlayer(player, outclient.BuildSystemMessageNoParams(outclient.SysMsgLearnedSkillS1))
	gl.sendToPlayer(player, gl.buildSkillListForPlayer(player))
	gl.sendToPlayer(player, outclient.BuildStatusUpdate(cmd.CharID, []outclient.StatusAttribute{
		{ID: outclient.StatusSP, Value: int32(char.SP)},
	}))

	// Refresh the learn window with the remaining learnable skills.
	gl.handleOpenSkillLearn(CmdOpenSkillLearn{CharID: cmd.CharID, NpcObjID: cmd.NpcObjID})

	log.Debug().Int32("char_id", cmd.CharID).Int32("skill", cmd.SkillID).Int32("level", cmd.Level).Msg("skill learned")
}

// buildSkillListForPlayer rebuilds the full SkillList (0x5F) from the live known
// skills, resolving the passive flag from the skill template (l2go-hv9).
func (gl *GameLoop) buildSkillListForPlayer(player *registry.PlayerWorldState) []byte {
	infos := make([]outclient.SkillInfo, 0, len(player.KnownSkills))
	for id, lvl := range player.KnownSkills {
		passive := false
		if gl.skillData != nil {
			if tmpl := gl.skillData.GetSkill(int(id), int(lvl)); tmpl != nil {
				passive = tmpl.IsPassive()
			}
		}
		infos = append(infos, outclient.SkillInfo{
			SkillID:     id,
			SkillLevel:  lvl,
			IsPassive:   passive,
			IsDisabled:  false,
			IsEnchanted: lvl > 100,
		})
	}
	return outclient.NewSkillList(infos)
}
