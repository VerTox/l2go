package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// Acquire-skill packets (L2J High Five, legacy 0x90/0x91/0x94; AcquireSkillType.CLASS
// ordinal = 0). SP-only flow: item requirements are not advertised. (l2go-hv9)

const acquireSkillTypeClass int32 = 0

// AcquireSkillEntry is one learnable skill row in AcquireSkillList.
type AcquireSkillEntry struct {
	ID     int32
	Level  int32
	SP     int32
	HasReq bool
}

// BuildAcquireSkillList (0x90) — the list of skills learnable at the trainer.
func BuildAcquireSkillList(skills []AcquireSkillEntry) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x90)
	w.WriteD(acquireSkillTypeClass)
	w.WriteD(int32(len(skills)))
	for _, s := range skills {
		w.WriteD(s.ID)
		w.WriteD(s.Level)
		w.WriteD(s.Level) // L2J repeats the level field
		w.WriteD(s.SP)
		if s.HasReq {
			w.WriteD(1)
		} else {
			w.WriteD(0)
		}
	}
	return w.Bytes()
}

// BuildAcquireSkillInfo (0x91) — details for one skill (SP cost, no item reqs).
func BuildAcquireSkillInfo(id, level, sp int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x91)
	w.WriteD(id)
	w.WriteD(level)
	w.WriteD(sp)
	w.WriteD(acquireSkillTypeClass)
	w.WriteD(0) // requirement count
	return w.Bytes()
}

// BuildAcquireSkillDone (0x94) — acknowledges a successful learn.
func BuildAcquireSkillDone() []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x94)
	return w.Bytes()
}
