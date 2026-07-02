package registry

import (
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// SkillEffectKind classifies the vital stat an item skill restores.
type SkillEffectKind int

const (
	EffectNone SkillEffectKind = iota
	EffectHP
	EffectMP
	EffectCP
)

// SkillEffect is the minimal slice of a skill the INTERIM potion handler needs:
// which vital stat to restore and by how much. It deliberately does NOT model the
// full L2J skill/effect system (durations, ticks, abnormals, land rate, ...) — it
// exists only so potions can immediately restore HP/MP/CP until a real skill engine
// (doSimultaneousCast) replaces it.
type SkillEffect struct {
	Kind   SkillEffectKind
	Amount int // flat restore amount (interim: tick effects apply a single "power")
}

// SkillEffectRegistry lazily reads L2J skill XML and extracts a single restore
// effect per (skill id, level). Files are located by the L2J range-naming
// convention ("02000-02099.xml") under any of the configured roots, also probing
// a "custom/" subdirectory (custom potion skills such as Mana Potion live there).
//
// This is NOT a skill engine: it only understands the restore effects potions use
// (Heal/Hp/Mp/Cp instant, TickHp/TickMp/TickCp over-time). Anything else resolves
// to EffectNone.
type SkillEffectRegistry struct {
	roots []string

	mu     sync.Mutex
	parsed map[int32]map[int]*xmlSkill // range-low -> (skillID -> skill); nil map = attempted, empty
}

// NewSkillEffectRegistry creates a registry that resolves skill files under the
// given root directories (tried in order).
func NewSkillEffectRegistry(roots []string) *SkillEffectRegistry {
	return &SkillEffectRegistry{
		roots:  roots,
		parsed: make(map[int32]map[int]*xmlSkill),
	}
}

// Lookup returns the restore effect for a skill id+level, if the skill exists and
// declares an effect the interim resolver understands.
func (r *SkillEffectRegistry) Lookup(skillID, level int) (SkillEffect, bool) {
	skill := r.skill(int32(skillID))
	if skill == nil {
		return SkillEffect{}, false
	}
	return skill.effect(level)
}

// skill returns the parsed skill, loading its range file on first access.
func (r *SkillEffectRegistry) skill(skillID int32) *xmlSkill {
	rangeLow := (skillID / 100) * 100

	r.mu.Lock()
	defer r.mu.Unlock()

	byID, ok := r.parsed[rangeLow]
	if !ok {
		byID = r.loadRange(rangeLow)
		r.parsed[rangeLow] = byID // cache even on failure (nil/empty) to avoid re-reading
	}
	return byID[int(skillID)]
}

// loadRange reads the XML file for a skill-id range from the first root that has it.
func (r *SkillEffectRegistry) loadRange(rangeLow int32) map[int]*xmlSkill {
	name := fmt.Sprintf("%05d-%05d.xml", rangeLow, rangeLow+99)
	for _, root := range r.roots {
		for _, candidate := range []string{
			filepath.Join(root, name),
			filepath.Join(root, "custom", name),
		} {
			data, err := os.ReadFile(candidate)
			if err != nil {
				continue
			}
			var list xmlSkillList
			if err := xml.Unmarshal(data, &list); err != nil {
				continue
			}
			out := make(map[int]*xmlSkill, len(list.Skills))
			for i := range list.Skills {
				s := &list.Skills[i]
				out[s.ID] = s
			}
			return out
		}
	}
	return map[int]*xmlSkill{}
}

// --- XML structures for L2J skill data ---

type xmlSkillList struct {
	XMLName xml.Name   `xml:"list"`
	Skills  []xmlSkill `xml:"skill"`
}

type xmlSkill struct {
	ID      int             `xml:"id,attr"`
	Levels  int             `xml:"levels,attr"`
	Name    string          `xml:"name,attr"`
	Tables  []xmlSkillTable `xml:"table"`
	Effects xmlSkillEffects `xml:"effects"`
}

type xmlSkillTable struct {
	Name   string `xml:"name,attr"`
	Values string `xml:",chardata"`
}

type xmlSkillEffects struct {
	Effects []xmlSkillEffect `xml:"effect"`
}

type xmlSkillEffect struct {
	Name   string          `xml:"name,attr"`
	Params []xmlSkillParam `xml:"param"`
}

// xmlSkillParam captures a <param key="value" /> element. In L2J skill XML the
// attribute NAME is the parameter key (e.g. <param power="8" />), so we read all
// attributes generically.
type xmlSkillParam struct {
	Attrs []xml.Attr `xml:",any,attr"`
}

// effect resolves the first understood restore effect for the given level.
func (s *xmlSkill) effect(level int) (SkillEffect, bool) {
	for _, e := range s.Effects.Effects {
		kind := effectKind(e.Name)
		if kind == EffectNone {
			continue
		}
		raw, ok := e.paramValue("amount", "power")
		if !ok {
			continue
		}
		amount, ok := s.resolveAmount(raw, level)
		if !ok {
			continue
		}
		return SkillEffect{Kind: kind, Amount: amount}, true
	}
	return SkillEffect{}, false
}

// paramValue returns the value of the first present param key (in priority order).
func (e xmlSkillEffect) paramValue(keys ...string) (string, bool) {
	for _, key := range keys {
		for _, attr := range e.Params {
			for _, a := range attr.Attrs {
				if a.Name.Local == key {
					return a.Value, true
				}
			}
		}
	}
	return "", false
}

// resolveAmount converts a raw param value to an int, resolving "#table" references
// by the (1-based) skill level and flooring fractional values (interim).
func (s *xmlSkill) resolveAmount(raw string, level int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "#") {
		vals := s.tableValues(raw)
		idx := level - 1
		if idx < 0 || idx >= len(vals) {
			return 0, false
		}
		raw = vals[idx]
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return int(math.Floor(f)), true
}

// tableValues returns the whitespace-separated values of the named table (e.g. "#amount").
func (s *xmlSkill) tableValues(name string) []string {
	for _, t := range s.Tables {
		if t.Name == name {
			return strings.Fields(t.Values)
		}
	}
	return nil
}

// effectKind maps an L2J effect name to the vital stat it restores. Unknown effect
// names (buffs, debuffs, damage, ...) resolve to EffectNone and are ignored.
func effectKind(name string) SkillEffectKind {
	switch name {
	case "Heal", "Hp", "TickHp", "HealPercent":
		return EffectHP
	case "Mp", "TickMp", "ManaHeal", "ManaHealPercent":
		return EffectMP
	case "Cp", "TickCp", "CpHeal", "CpHealPercent":
		return EffectCP
	default:
		return EffectNone
	}
}
