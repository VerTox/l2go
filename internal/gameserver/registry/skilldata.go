package registry

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/VerTox/l2go/internal/gameserver/models"
)

// SkillHashCode is L2J's centralized (id, level) -> int key: id*1021 + level.
// One Skill object exists per hash. Enchanted levels (>100) are out of scope for P0.
func SkillHashCode(skillID, level int) int { return skillID*1021 + level }

// SkillData is the in-memory skill template registry. It lazily parses the L2J
// skill XML (data/stats/skills/NNNNN-NNNNN.xml) one id-range file at a time and
// expands every <skill> into one models.Skill per declared level. Lookups clamp a
// too-high level down to the skill's max, matching L2J SkillData.getSkill.
//
// This replaces the parsing role of the interim SkillEffectRegistry: it models the
// full skill template (operate type, costs, target, effects) rather than just the
// single restore effect potions needed. Effects are COLLECTED, not executed (P0).
type SkillData struct {
	roots []string

	mu       sync.Mutex
	loaded   map[int32]bool         // rangeLow -> range file parsed (even if empty/missing)
	skills   map[int]*models.Skill  // SkillHashCode -> skill
	maxLevel map[int]int            // skillID -> highest non-enchant level present
}

// NewSkillData creates a registry resolving skill files under the given roots (in order).
func NewSkillData(roots []string) *SkillData {
	return &SkillData{
		roots:    roots,
		loaded:   make(map[int32]bool),
		skills:   make(map[int]*models.Skill),
		maxLevel: make(map[int]int),
	}
}

// GetSkill returns the skill template for (id, level), loading its range file on
// first access. A level above the skill's max clamps down to the max (L2J parity).
// Returns nil if the skill id is unknown.
func (r *SkillData) GetSkill(skillID, level int) *models.Skill {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureRange(int32(skillID))

	if s := r.skills[SkillHashCode(skillID, level)]; s != nil {
		return s
	}
	// Requested level too high: fall back to the max known level.
	if max := r.maxLevel[skillID]; max > 0 && level > max {
		return r.skills[SkillHashCode(skillID, max)]
	}
	return nil
}

// MaxLevel returns the highest level parsed for a skill id (0 if unknown), loading
// the range file on first access.
func (r *SkillData) MaxLevel(skillID int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureRange(int32(skillID))
	return r.maxLevel[skillID]
}

// ensureRange parses the id-range file containing skillID once. Caller holds r.mu.
func (r *SkillData) ensureRange(skillID int32) {
	rangeLow := (skillID / 100) * 100
	if r.loaded[rangeLow] {
		return
	}
	r.loaded[rangeLow] = true // mark first: a missing/broken file must not retry every lookup

	skills := r.loadRange(rangeLow)
	for _, s := range skills {
		r.skills[SkillHashCode(s.ID, s.Level)] = s
		if s.Level > r.maxLevel[s.ID] {
			r.maxLevel[s.ID] = s.Level
		}
	}
}

// loadRange reads and parses the XML file for a skill-id range from the first root
// that has it, returning every expanded (id, level) skill. Missing files yield nil.
func (r *SkillData) loadRange(rangeLow int32) []*models.Skill {
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
			skills, err := parseSkillList(data)
			if err != nil {
				continue
			}
			return skills
		}
	}
	return nil
}

// --- XML model (raw datapack shapes) ---

type xmlSkillDoc struct {
	XMLName xml.Name       `xml:"list"`
	Skills  []xmlSkillNode `xml:"skill"`
}

type xmlSkillNode struct {
	ID     int          `xml:"id,attr"`
	Levels int          `xml:"levels,attr"`
	Name   string       `xml:"name,attr"`
	Tables []xmlTable    `xml:"table"`
	Sets   []xmlSkillSet `xml:"set"`
	// EffectScope wrappers. P0 collects effects from all present scopes.
	Effects           xmlEffectScope `xml:"effects"`
	SelfEffects       xmlEffectScope `xml:"selfEffects"`
	StartEffects      xmlEffectScope `xml:"startEffects"`
	EndEffects        xmlEffectScope `xml:"endEffects"`
	PveEffects        xmlEffectScope `xml:"pveEffects"`
	PvpEffects        xmlEffectScope `xml:"pvpEffects"`
	ChannelingEffects xmlEffectScope `xml:"channelingEffects"`
}

type xmlTable struct {
	Name   string `xml:"name,attr"`
	Values string `xml:",chardata"`
}

type xmlSkillSet struct {
	Name string `xml:"name,attr"`
	Val  string `xml:"val,attr"`
}

type xmlEffectScope struct {
	Effects []xmlEffectNode `xml:"effect"`
}

type xmlEffectNode struct {
	Name   string       `xml:"name,attr"`
	Params []xmlAnyAttr `xml:"param"`
	Add    []xmlFuncSet `xml:"add"`
	Sub    []xmlFuncSet `xml:"sub"`
	Mul    []xmlFuncSet `xml:"mul"`
	Div    []xmlFuncSet `xml:"div"`
	Set    []xmlFuncSet `xml:"set"`
}

// xmlAnyAttr captures every attribute of a <param .../> element generically; the
// attribute NAME is the parameter key (e.g. <param power="#healPower" />).
type xmlAnyAttr struct {
	Attrs []xml.Attr `xml:",any,attr"`
}

type xmlFuncSet struct {
	Stat  string `xml:"stat,attr"`
	Val   string `xml:"val,attr"`
	Order string `xml:"order,attr"`
}

// parseSkillList unmarshals a skill-list XML document and expands each <skill> into
// one models.Skill per declared level, resolving #table references per level.
func parseSkillList(data []byte) ([]*models.Skill, error) {
	var doc xmlSkillDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var out []*models.Skill
	for i := range doc.Skills {
		out = append(out, expandSkill(&doc.Skills[i])...)
	}
	return out, nil
}

// expandSkill produces one Skill per level (1..Levels) from a raw <skill> node.
func expandSkill(n *xmlSkillNode) []*models.Skill {
	levels := n.Levels
	if levels < 1 {
		levels = 1
	}
	tables := make(map[string][]string, len(n.Tables))
	for _, t := range n.Tables {
		tables[t.Name] = strings.Fields(t.Values)
	}

	skills := make([]*models.Skill, 0, levels)
	for lvl := 1; lvl <= levels; lvl++ {
		// Resolve <set> bean values for this level into a flat map.
		stats := make(map[string]string, len(n.Sets))
		for _, s := range n.Sets {
			stats[s.Name] = resolveTableRef(s.Val, tables, lvl)
		}

		sk := &models.Skill{
			ID:           n.ID,
			Level:        lvl,
			DisplayID:    intStat(stats, "displayId", n.ID),
			DisplayLevel: intStat(stats, "displayLevel", lvl),
			Name:         n.Name,
			OperateType:  models.SkillOperateType(stats["operateType"]),
			Magic:        intStat(stats, "isMagic", 0),
			TargetType:   targetTypeStat(stats, "targetType", models.TargetSelf),
			CastRange:    intStat(stats, "castRange", -1),
			EffectRange:  intStat(stats, "effectRange", -1),
			HitTime:      intStat(stats, "hitTime", 0),
			CoolTime:     intStat(stats, "coolTime", 0),
			ReuseDelay:   intStat(stats, "reuseDelay", 0),
			MpConsume1:   intStat(stats, "mpConsume1", 0),
			MpConsume2:   intStat(stats, "mpConsume2", 0),
			HpConsume:    intStat(stats, "hpConsume", 0),
			ItemConsumeID:    intStat(stats, "itemConsumeId", 0),
			ItemConsumeCount: intStat(stats, "itemConsumeCount", 0),
			EffectPoint:  intStat(stats, "effectPoint", 0),
			AbnormalType: abnormalTypeStat(stats, "abnormalType", models.AbnormalNone),
			AbnormalLvl:  intStat(stats, "abnormalLvl", 0),
			AbnormalTime: intStat(stats, "abnormalTime", 0),
			IsDebuff:     boolStat(stats, "isDebuff", false),
			MagicLevel:   intStat(stats, "magicLvl", 0),
			ActivateRate: intStat(stats, "activateRate", -1),
		}

		// A skill with no explicit scope wrapper falls back to PASSIVE (if passive)
		// or GENERAL, mirroring L2J attachEffect.
		generalScope := models.ScopeGeneral
		if sk.OperateType.IsPassive() {
			generalScope = models.ScopePassive
		}
		sk.Effects = collectEffects(n, tables, lvl, generalScope)

		skills = append(skills, sk)
	}
	return skills
}

// collectEffects gathers effects from every scope wrapper, resolving table refs at
// the given level.
func collectEffects(n *xmlSkillNode, tables map[string][]string, lvl int, generalScope models.EffectScope) []models.SkillEffect {
	var effects []models.SkillEffect
	add := func(nodes []xmlEffectNode, scope models.EffectScope) {
		for _, e := range nodes {
			effects = append(effects, buildEffect(e, tables, lvl, scope))
		}
	}
	add(n.Effects.Effects, generalScope)
	add(n.SelfEffects.Effects, models.ScopeSelf)
	add(n.StartEffects.Effects, models.ScopeStart)
	add(n.EndEffects.Effects, models.ScopeEnd)
	add(n.PveEffects.Effects, models.ScopePve)
	add(n.PvpEffects.Effects, models.ScopePvp)
	add(n.ChannelingEffects.Effects, models.ScopeChanneling)
	return effects
}

// buildEffect resolves one <effect>'s params and stat funcs for a level.
func buildEffect(e xmlEffectNode, tables map[string][]string, lvl int, scope models.EffectScope) models.SkillEffect {
	eff := models.SkillEffect{Name: e.Name, Scope: scope}
	for _, p := range e.Params {
		for _, a := range p.Attrs {
			if eff.Params == nil {
				eff.Params = make(map[string]string)
			}
			eff.Params[a.Name.Local] = resolveTableRef(a.Value, tables, lvl)
		}
	}
	appendFuncs := func(op string, fns []xmlFuncSet) {
		for _, f := range fns {
			val, ok := parseFloatRef(f.Val, tables, lvl)
			if !ok {
				continue
			}
			eff.Funcs = append(eff.Funcs, models.SkillFunc{
				Op:    op,
				Stat:  f.Stat,
				Val:   val,
				Order: parseOrder(f.Order),
			})
		}
	}
	appendFuncs("add", e.Add)
	appendFuncs("sub", e.Sub)
	appendFuncs("mul", e.Mul)
	appendFuncs("div", e.Div)
	appendFuncs("set", e.Set)
	return eff
}

// resolveTableRef returns raw unless it is a "#table" reference, in which case it
// returns the table's value for the (1-based) level, or "" if out of range.
func resolveTableRef(raw string, tables map[string][]string, level int) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "#") {
		return raw
	}
	vals := tables[raw]
	idx := level - 1
	if idx < 0 || idx >= len(vals) {
		return ""
	}
	return vals[idx]
}

// parseFloatRef resolves a possibly-tabled value to a float.
func parseFloatRef(raw string, tables map[string][]string, level int) (float64, bool) {
	s := resolveTableRef(raw, tables, level)
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func parseOrder(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	// order values are sometimes hex (0x..) in L2J.
	if v, err := strconv.ParseInt(s, 0, 64); err == nil {
		return int(v)
	}
	return -1
}

// intStat reads an int from the resolved set map, flooring fractional values and
// falling back to def when absent/unparseable.
func intStat(stats map[string]string, key string, def int) int {
	s, ok := stats[key]
	if !ok || s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f)
	}
	return def
}

func boolStat(stats map[string]string, key string, def bool) bool {
	s, ok := stats[key]
	if !ok || s == "" {
		return def
	}
	if v, err := strconv.ParseBool(s); err == nil {
		return v
	}
	return def
}

func targetTypeStat(stats map[string]string, key string, def models.TargetType) models.TargetType {
	if s, ok := stats[key]; ok && s != "" {
		return models.TargetType(s)
	}
	return def
}

func abnormalTypeStat(stats map[string]string, key string, def models.AbnormalType) models.AbnormalType {
	if s, ok := stats[key]; ok && s != "" {
		return models.AbnormalType(s)
	}
	return def
}
