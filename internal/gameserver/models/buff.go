package models

import "time"

// TickKind is the vital a periodic (HoT/DoT) buff tick affects.
type TickKind int

const (
	TickNone TickKind = iota
	TickHP
	TickMP
)

// BuffTick is a periodic effect of a buff (L2J TickHp/TickMp). Power is applied
// every IntervalSec seconds; a negative power is damage (DoT). Fatal marks
// TickHpFatal (a DoT that cannot reduce HP below 1).
type BuffTick struct {
	Kind        TickKind
	Power       int
	IntervalSec int
	Fatal       bool
}

// BuffInfo is a runtime instance of a continuous skill effect on a character
// (L2J BuffInfo). It carries the stat modifiers the buff contributes and any
// periodic ticks, plus the runtime expiry/tick schedule set by the game loop.
type BuffInfo struct {
	SkillID      int32
	SkillLevel   int32
	DisplayID    int32
	DisplayLevel int32
	AbnormalType AbnormalType
	AbnormalLvl  int
	DurationSec  int // abnormalTime; 0 = infinite (toggle)
	Toggle       bool

	Mods  []StatModifier
	Ticks []BuffTick

	// Runtime schedule (game-loop owned).
	ExpiresAt time.Time // zero = infinite
	NextTick  time.Time // next HoT/DoT tick (zero if no ticks)
}

// HasTicks reports whether the buff has any periodic effect.
func (b *BuffInfo) HasTicks() bool { return len(b.Ticks) > 0 }

// TickInterval returns the buff's tick period (the first tick's interval; a skill's
// ticks share a cadence in practice). Zero if the buff has no ticks.
func (b *BuffInfo) TickInterval() time.Duration {
	if len(b.Ticks) == 0 || b.Ticks[0].IntervalSec <= 0 {
		return 0
	}
	return time.Duration(b.Ticks[0].IntervalSec) * time.Second
}

// CharEffectList holds a character's active continuous effects (buffs/debuffs/
// toggles), applying L2J abnormal-type stacking rules on insert.
type CharEffectList struct {
	buffs []*BuffInfo
}

// Buffs returns the active buffs (live slice — do not mutate).
func (l *CharEffectList) Buffs() []*BuffInfo { return l.buffs }

// Len returns the number of active buffs.
func (l *CharEffectList) Len() int { return len(l.buffs) }

// HasSkill reports whether a skill id is currently active.
func (l *CharEffectList) HasSkill(skillID int32) bool {
	return l.indexOfSkill(skillID) >= 0
}

// Add inserts a buff, applying stacking rules, and returns whether it was added:
//   - same skill id always refreshes (old instance removed first);
//   - same non-NONE abnormalType: the new buff replaces the old only if its
//     abnormalLvl >= the old one's; a weaker buff is rejected (not added).
//
// Returns false only when a stronger same-type buff is already present.
func (l *CharEffectList) Add(b *BuffInfo) bool {
	// Refresh: drop any existing instance of the same skill.
	if i := l.indexOfSkill(b.SkillID); i >= 0 {
		l.removeAt(i)
	}

	if b.AbnormalType != "" && b.AbnormalType != AbnormalNone {
		if i := l.indexOfType(b.AbnormalType); i >= 0 {
			if b.AbnormalLvl < l.buffs[i].AbnormalLvl {
				return false // a stronger buff of this type is active
			}
			l.removeAt(i) // replace the weaker/equal one
		}
	}

	l.buffs = append(l.buffs, b)
	return true
}

// RemoveSkill removes the buff for a skill id, returning true if present.
func (l *CharEffectList) RemoveSkill(skillID int32) bool {
	i := l.indexOfSkill(skillID)
	if i < 0 {
		return false
	}
	l.removeAt(i)
	return true
}

// RemoveExpired drops buffs whose ExpiresAt has passed, returning them.
func (l *CharEffectList) RemoveExpired(now time.Time) []*BuffInfo {
	var expired []*BuffInfo
	kept := l.buffs[:0]
	for _, b := range l.buffs {
		if !b.ExpiresAt.IsZero() && !now.Before(b.ExpiresAt) {
			expired = append(expired, b)
			continue
		}
		kept = append(kept, b)
	}
	l.buffs = kept
	return expired
}

// Mods returns the combined stat modifiers of all active buffs.
func (l *CharEffectList) Mods() []StatModifier {
	var mods []StatModifier
	for _, b := range l.buffs {
		mods = append(mods, b.Mods...)
	}
	return mods
}

func (l *CharEffectList) indexOfSkill(skillID int32) int {
	for i, b := range l.buffs {
		if b.SkillID == skillID {
			return i
		}
	}
	return -1
}

func (l *CharEffectList) indexOfType(t AbnormalType) int {
	for i, b := range l.buffs {
		if b.AbnormalType == t {
			return i
		}
	}
	return -1
}

func (l *CharEffectList) removeAt(i int) {
	l.buffs = append(l.buffs[:i], l.buffs[i+1:]...)
}
