package registry

import (
	"encoding/xml"
	"os"
	"sort"
	"sync"
)

// AutoGetSkill is a single skill a class receives automatically at a level.
type AutoGetSkill struct {
	SkillID int32
	Level   int
}

// classTreeEntry is one raw <skill> row of a class skill tree.
type classTreeEntry struct {
	SkillID  int32
	SkillLvl int
	GetLevel int
	AutoGet  bool
}

// SkillTreeData holds the per-class skill trees parsed from classSkillTree.xml.
// A class inherits its parent's tree (parentClassId), so the effective tree is the
// union up the class chain. Only the auto-get subset is consumed for now (the
// full learn-by-NPC flow with SP costs is a later phase, l2go-hv9).
type SkillTreeData struct {
	mu     sync.RWMutex
	trees  map[int][]classTreeEntry // classId -> own entries
	parent map[int]int              // classId -> parentClassId (absent = root)
	loaded bool
}

// NewSkillTreeData creates an empty registry.
func NewSkillTreeData() *SkillTreeData {
	return &SkillTreeData{
		trees:  make(map[int][]classTreeEntry),
		parent: make(map[int]int),
	}
}

// Global instance (mirrors item/npc template registries).
var skillTrees = NewSkillTreeData()

// GetSkillTreeRegistry returns the global class skill tree registry.
func GetSkillTreeRegistry() *SkillTreeData { return skillTrees }

// IsLoaded reports whether a tree file has been parsed.
func (r *SkillTreeData) IsLoaded() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded
}

// LoadFromFile parses a classSkillTree.xml file into the registry, replacing any
// previously loaded data.
func (r *SkillTreeData) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return r.load(data)
}

func (r *SkillTreeData) load(data []byte) error {
	var doc xmlSkillTreeList
	if err := xml.Unmarshal(data, &doc); err != nil {
		return err
	}

	trees := make(map[int][]classTreeEntry)
	parent := make(map[int]int)
	for _, t := range doc.Trees {
		if t.Type != "" && t.Type != "classSkillTree" {
			continue
		}
		if t.ParentClassID != nil && *t.ParentClassID != t.ClassID {
			parent[t.ClassID] = *t.ParentClassID
		}
		entries := make([]classTreeEntry, 0, len(t.Skills))
		for _, s := range t.Skills {
			entries = append(entries, classTreeEntry{
				SkillID:  s.SkillID,
				SkillLvl: s.SkillLvl,
				GetLevel: s.GetLevel,
				AutoGet:  s.AutoGet,
			})
		}
		trees[t.ClassID] = entries
	}

	r.mu.Lock()
	r.trees, r.parent, r.loaded = trees, parent, true
	r.mu.Unlock()
	return nil
}

// AutoGetSkills returns the auto-get skills a character of the given class should
// have at the given level: every autoGet entry (in the class and its parent chain)
// whose getLevel <= level, deduped to the highest skill level per skill id. Mirrors
// L2J SkillTreesData.getAvailableAutoGetSkills over the complete (inherited) tree.
func (r *SkillTreeData) AutoGetSkills(classID, level int) []AutoGetSkill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	best := make(map[int32]int) // skillId -> highest applicable skill level
	seen := make(map[int]bool)  // guard against parent cycles
	for cid := classID; ; {
		if seen[cid] {
			break
		}
		seen[cid] = true
		for _, e := range r.trees[cid] {
			if !e.AutoGet || level < e.GetLevel {
				continue
			}
			if e.SkillLvl > best[e.SkillID] {
				best[e.SkillID] = e.SkillLvl
			}
		}
		p, ok := r.parent[cid]
		if !ok {
			break
		}
		cid = p
	}

	out := make([]AutoGetSkill, 0, len(best))
	for id, lvl := range best {
		out = append(out, AutoGetSkill{SkillID: id, Level: lvl})
	}
	// Deterministic order (map iteration is random) — callers persist/compare these.
	sort.Slice(out, func(i, j int) bool { return out[i].SkillID < out[j].SkillID })
	return out
}

// --- XML shapes ---

type xmlSkillTreeList struct {
	XMLName xml.Name          `xml:"list"`
	Trees   []xmlClassSkillTree `xml:"skillTree"`
}

type xmlClassSkillTree struct {
	Type          string             `xml:"type,attr"`
	ClassID       int                `xml:"classId,attr"`
	ParentClassID *int               `xml:"parentClassId,attr"`
	Skills        []xmlTreeSkill     `xml:"skill"`
}

type xmlTreeSkill struct {
	SkillID  int32 `xml:"skillId,attr"`
	SkillLvl int   `xml:"skillLvl,attr"`
	GetLevel int   `xml:"getLevel,attr"`
	AutoGet  bool  `xml:"autoGet,attr"`
}
