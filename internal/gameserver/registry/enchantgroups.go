package registry

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// enchantRange is one row of an enchant rate group: enchant levels [min,max] map
// to a success chance in percent. Mirrors L2J's RangeChanceHolder.
type enchantRange struct {
	min, max int
	chance   float64
}

// enchantRateItem is one binding inside an enchant scroll group: it selects a
// named rate group when the target item matches the (optional) itemId / slot mask
// / magic-weapon conditions. Mirrors L2J's EnchantRateItem.
type enchantRateItem struct {
	group       string // name of the enchantRateGroup to apply
	itemID      int32  // 0 => any item
	slot        int32  // OR of body-part bits; 0 => any slot
	magicWeapon *bool  // nil => any; else must match target's magic-weapon flag
}

// validate reports whether the target item (its body-part bitmask, magic-weapon
// flag and id) can use this rate binding. Mirrors EnchantRateItem.validate.
func (ri enchantRateItem) validate(bodyPart int32, isMagicWeapon bool, itemID int32) bool {
	if ri.itemID != 0 && ri.itemID != itemID {
		return false
	}
	if ri.slot != 0 && (bodyPart&ri.slot) == 0 {
		return false
	}
	return ri.magicWeapon == nil || (*ri.magicWeapon == isMagicWeapon)
}

// EnchantGroupsRegistry holds the enchant rate groups (chance-by-level tables)
// and scroll groups (target -> rate-group bindings) parsed from
// enchantItemGroups.xml. It is the retail-accurate source of enchant success
// chance, replacing hardcoded tables.
type EnchantGroupsRegistry struct {
	mu           sync.RWMutex
	rateGroups   map[string][]enchantRange   // group name -> chance ranges
	scrollGroups map[int][]enchantRateItem   // scroll group id -> ordered bindings
}

// NewEnchantGroupsRegistry creates an empty enchant-groups registry.
func NewEnchantGroupsRegistry() *EnchantGroupsRegistry {
	return &EnchantGroupsRegistry{
		rateGroups:   make(map[string][]enchantRange),
		scrollGroups: make(map[int][]enchantRateItem),
	}
}

// RateGroupCount / ScrollGroupCount expose the loaded counts (tests/logging).
func (r *EnchantGroupsRegistry) RateGroupCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.rateGroups)
}
func (r *EnchantGroupsRegistry) ScrollGroupCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.scrollGroups)
}

// Chance returns the success chance (percent) for enchanting an item at
// enchantLevel using the scroll's scrollGroupId. The item is described by its
// body-part bitmask, magic-weapon flag and id (for id-specific bindings). ok is
// false when the scroll group, a matching rate binding, or the rate group is
// missing — mirroring L2J's getItemGroup returning null / getChance returning -1.
func (r *EnchantGroupsRegistry) Chance(scrollGroupID int, bodyPart int32, isMagicWeapon bool, itemID int32, enchantLevel int) (float64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bindings, ok := r.scrollGroups[scrollGroupID]
	if !ok {
		return 0, false
	}
	for _, ri := range bindings {
		if !ri.validate(bodyPart, isMagicWeapon, itemID) {
			continue
		}
		ranges, ok := r.rateGroups[ri.group]
		if !ok || len(ranges) == 0 {
			return 0, false
		}
		return chanceForLevel(ranges, enchantLevel), true
	}
	return 0, false
}

// chanceForLevel returns the chance for the given enchant level, falling back to
// the last range if none matches (mirrors EnchantItemGroup.getChance).
func chanceForLevel(ranges []enchantRange, level int) float64 {
	for _, rg := range ranges {
		if level >= rg.min && level <= rg.max {
			return rg.chance
		}
	}
	return ranges[len(ranges)-1].chance
}

// --- XML parsing ---------------------------------------------------------

type xmlEnchantGroupsList struct {
	XMLName      xml.Name              `xml:"list"`
	RateGroups   []xmlEnchantRateGroup `xml:"enchantRateGroup"`
	ScrollGroups []xmlEnchantScroll    `xml:"enchantScrollGroup"`
}

type xmlEnchantRateGroup struct {
	Name     string             `xml:"name,attr"`
	Currents []xmlEnchantCurrent `xml:"current"`
}

type xmlEnchantCurrent struct {
	Enchant string `xml:"enchant,attr"`
	Chance  string `xml:"chance,attr"`
}

type xmlEnchantScroll struct {
	ID    int                `xml:"id,attr"`
	Rates []xmlEnchantRateEl `xml:"enchantRate"`
}

type xmlEnchantRateEl struct {
	Group string              `xml:"group,attr"`
	Items []xmlEnchantRateSub `xml:"item"`
}

type xmlEnchantRateSub struct {
	Slot        string `xml:"slot,attr"`
	MagicWeapon string `xml:"magicWeapon,attr"`
	ID          string `xml:"id,attr"`
}

// LoadFromFile loads enchant rate/scroll groups from the first existing path
// among the candidates (same probing convention as the other data registries).
func (r *EnchantGroupsRegistry) LoadFromFile(candidates ...string) error {
	var lastErr error
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		var list xmlEnchantGroupsList
		if err := xml.Unmarshal(data, &list); err != nil {
			return fmt.Errorf("parse enchant groups %s: %w", path, err)
		}

		rateGroups := make(map[string][]enchantRange, len(list.RateGroups))
		for _, rg := range list.RateGroups {
			var ranges []enchantRange
			for _, c := range rg.Currents {
				min, max, ok := parseEnchantRange(c.Enchant)
				if !ok {
					continue
				}
				ranges = append(ranges, enchantRange{min: min, max: max, chance: atofDefault(c.Chance, 0)})
			}
			rateGroups[rg.Name] = ranges
		}

		scrollGroups := make(map[int][]enchantRateItem, len(list.ScrollGroups))
		for _, sg := range list.ScrollGroups {
			bindings := make([]enchantRateItem, 0, len(sg.Rates))
			for _, rate := range sg.Rates {
				ri := enchantRateItem{group: rate.Group}
				for _, it := range rate.Items {
					if it.Slot != "" {
						ri.slot |= slotStringToMask(it.Slot)
					}
					if it.MagicWeapon != "" {
						mw := strings.EqualFold(strings.TrimSpace(it.MagicWeapon), "true")
						ri.magicWeapon = &mw
					}
					if it.ID != "" {
						ri.itemID = int32(atoiDefault(it.ID, 0))
					}
				}
				bindings = append(bindings, ri)
			}
			scrollGroups[sg.ID] = bindings
		}

		r.mu.Lock()
		r.rateGroups = rateGroups
		r.scrollGroups = scrollGroups
		r.mu.Unlock()

		log.Info().Str("file", path).
			Int("rate_groups", len(rateGroups)).
			Int("scroll_groups", len(scrollGroups)).
			Msg("Enchant item groups loaded")
		return nil
	}
	return fmt.Errorf("no enchant groups file found: %w", lastErr)
}

// parseEnchantRange parses "0-2" / "5" / "20-65535" the way L2J does: single
// values yield min==max; the entry is dropped unless min>-1 && max>0 (so a bare
// "0" is skipped, matching EnchantItemGroupsData).
func parseEnchantRange(s string) (int, int, bool) {
	s = strings.TrimSpace(s)
	min, max := -1, 0
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		lo, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		hi, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 == nil && err2 == nil {
			min, max = lo, hi
		}
	} else if v, err := strconv.Atoi(s); err == nil {
		min, max = v, v
	}
	if min > -1 && max > 0 {
		return min, max, true
	}
	return 0, 0, false
}

// slotStringToMask maps an enchantItemGroups.xml slot token to a body-part
// bitmask, mirroring L2J's ItemTable.SLOTS (including the combined "rear;lear" /
// "rfinger;lfinger" keys). Bit values match bodyPartToCode so masks compare
// directly against a target template's BodyPartCode.
func slotStringToMask(slot string) int32 {
	switch strings.ToLower(strings.TrimSpace(slot)) {
	case "underwear", "shirt":
		return 0x0001
	case "rear":
		return 0x0002
	case "lear":
		return 0x0004
	case "rear;lear":
		return 0x0002 | 0x0004
	case "neck":
		return 0x0008
	case "rfinger":
		return 0x0010
	case "lfinger":
		return 0x0020
	case "rfinger;lfinger":
		return 0x0010 | 0x0020
	case "head":
		return 0x0040
	case "rhand":
		return 0x0080
	case "lhand":
		return 0x0100
	case "gloves":
		return 0x0200
	case "chest":
		return 0x0400
	case "legs":
		return 0x0800
	case "chest,legs":
		return 0x0400 | 0x0800
	case "feet":
		return 0x1000
	case "back", "cloak":
		return 0x2000
	case "lrhand":
		return 0x4000
	case "onepiece", "fullarmor":
		return 0x8000
	case "hair":
		return 0x010000
	case "alldress":
		return 0x020000
	case "hair2", "dhair":
		return 0x040000
	case "hairall":
		return 0x050000
	case "rbracelet":
		return 0x100000
	case "lbracelet":
		return 0x200000
	case "deco", "deco1", "talisman":
		return 0x400000
	case "belt", "waist":
		return 0x10000000
	default:
		// Unknown token contributes no bits (never matches), matching a missing
		// SLOTS entry in L2J.
		return 0
	}
}
