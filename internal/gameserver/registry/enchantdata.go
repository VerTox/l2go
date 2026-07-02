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

// EnchantScrollData carries the per-scroll tuning parsed from enchantItemData.xml
// (L2J EnchantScroll / AbstractEnchantItem): which item grade the scroll targets,
// its scroll-group id, the maximum enchant level it can be used at, and any flat
// bonus chance it grants.
type EnchantScrollData struct {
	TargetGrade   ItemGrade
	ScrollGroupID int
	MaxEnchant    int
	BonusRate     float64
}

// EnchantDataRegistry holds enchant scroll tuning keyed by scroll item id.
type EnchantDataRegistry struct {
	mu      sync.RWMutex
	scrolls map[int32]EnchantScrollData
}

// NewEnchantDataRegistry creates an empty enchant-data registry.
func NewEnchantDataRegistry() *EnchantDataRegistry {
	return &EnchantDataRegistry{scrolls: make(map[int32]EnchantScrollData)}
}

// GetEnchantScroll returns the tuning data for a scroll item id, if present.
func (r *EnchantDataRegistry) GetEnchantScroll(itemID int32) (EnchantScrollData, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.scrolls[itemID]
	return d, ok
}

// Count returns the number of loaded scrolls.
func (r *EnchantDataRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.scrolls)
}

// Set inserts/overrides a scroll entry (used by loaders and tests).
func (r *EnchantDataRegistry) Set(itemID int32, d EnchantScrollData) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scrolls[itemID] = d
}

// XML shape of data/enchantItemData.xml.
type xmlEnchantList struct {
	XMLName  xml.Name          `xml:"list"`
	Enchants []xmlEnchantEntry `xml:"enchant"`
}

type xmlEnchantEntry struct {
	ID            int32  `xml:"id,attr"`
	TargetGrade   string `xml:"targetGrade,attr"`
	ScrollGroupID string `xml:"scrollGroupId,attr"`
	MaxEnchant    string `xml:"maxEnchant,attr"`
	BonusRate     string `xml:"bonusRate,attr"`
}

// LoadFromFile loads enchant scroll tuning from the first existing path among the
// given candidates (mirrors how other registries probe data/ vs references/data).
func (r *EnchantDataRegistry) LoadFromFile(candidates ...string) error {
	var lastErr error
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		var list xmlEnchantList
		if err := xml.Unmarshal(data, &list); err != nil {
			return fmt.Errorf("parse enchant data %s: %w", path, err)
		}

		r.mu.Lock()
		count := 0
		for _, e := range list.Enchants {
			if e.ID == 0 {
				continue
			}
			r.scrolls[e.ID] = EnchantScrollData{
				TargetGrade:   parseCrystalType(e.TargetGrade),
				ScrollGroupID: atoiDefault(e.ScrollGroupID, 0),
				MaxEnchant:    atoiDefault(e.MaxEnchant, 65535),
				BonusRate:     atofDefault(e.BonusRate, 0),
			}
			count++
		}
		r.mu.Unlock()

		log.Info().Str("file", path).Int("scrolls", count).Msg("Enchant item data loaded")
		return nil
	}
	return fmt.Errorf("no enchant data file found: %w", lastErr)
}

func atoiDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

func atofDefault(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return def
}
