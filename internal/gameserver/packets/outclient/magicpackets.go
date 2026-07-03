package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// BuildMagicSkillLaunched builds MagicSkillLaunched (opcode 0x54) — sent at the
// launch point of a cast (after the cast animation, before the effect) with the
// resolved target list. L2J MagicSkillLaunched.writeImpl:
//
//	C  0x54
//	D  caster object id
//	D  skill id
//	D  skill level
//	D  target count
//	D* target object ids
func BuildMagicSkillLaunched(casterObjectID, skillID, skillLevel int32, targets []int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x54)
	w.WriteD(casterObjectID)
	w.WriteD(skillID)
	w.WriteD(skillLevel)
	w.WriteD(int32(len(targets)))
	for _, t := range targets {
		w.WriteD(t)
	}
	return w.Bytes()
}

// SetupGauge colors (the cast/action bar tint).
const (
	GaugeColorBlue  int32 = 0
	GaugeColorRed   int32 = 1
	GaugeColorCyan  int32 = 2
	GaugeColorGreen int32 = 3
)

// BuildSetupGauge builds SetupGauge (opcode 0x6b) — the coloured progress bar over
// the caster during a cast. L2J SetupGauge.writeImpl:
//
//	C  0x6b
//	D  char object id
//	D  colour
//	D  current time (ms)
//	D  max time (ms)
func BuildSetupGauge(charObjectID, color, time int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x6b)
	w.WriteD(charObjectID)
	w.WriteD(color)
	w.WriteD(time)
	w.WriteD(time)
	return w.Bytes()
}

// BuildMagicSkillCanceled builds MagicSkillCanceled (opcode 0x49) — sent when a
// cast is aborted (movement/damage/stun). L2J MagicSkillCanceled.writeImpl:
//
//	C  0x49
//	D  object id
func BuildMagicSkillCanceled(objectID int32) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x49)
	w.WriteD(objectID)
	return w.Bytes()
}

// SkillReuseEntry is one skill's cooldown state for SkillCoolTime.
type SkillReuseEntry struct {
	SkillID      int32
	SkillLevel   int32
	ReuseMillis  int32 // total reuse
	RemainMillis int32 // remaining
}

// BuildSkillCoolTime builds SkillCoolTime (opcode 0xC7) — the client-side skill
// cooldown sweeps. L2J SkillCoolTime.writeImpl (reuse/remaining are in seconds):
//
//	C  0xC7
//	D  count
//	per entry: D skillId, D skillLevel, D reuse(sec), D remaining(sec)
func BuildSkillCoolTime(entries []SkillReuseEntry) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0xc7)
	w.WriteD(int32(len(entries)))
	for _, e := range entries {
		w.WriteD(e.SkillID)
		w.WriteD(e.SkillLevel)
		w.WriteD(e.ReuseMillis / 1000)
		w.WriteD(e.RemainMillis / 1000)
	}
	return w.Bytes()
}
