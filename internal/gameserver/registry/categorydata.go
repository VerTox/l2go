package registry

import (
	"encoding/xml"
	"os"
	"sync"
)

// CategoryData holds class groupings parsed from categoryData.xml (L2J CategoryData):
// category name -> set of class ids. Used to gate NPC-trainer skill learning by the
// player's class category (l2go-hv9).
type CategoryData struct {
	mu         sync.RWMutex
	categories map[string]map[int]bool
	loaded     bool
}

// NewCategoryData creates an empty registry.
func NewCategoryData() *CategoryData {
	return &CategoryData{categories: make(map[string]map[int]bool)}
}

var categoryData = NewCategoryData()

// GetCategoryRegistry returns the global category registry.
func GetCategoryRegistry() *CategoryData { return categoryData }

// IsLoaded reports whether a category file has been parsed.
func (c *CategoryData) IsLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loaded
}

// LoadFromFile parses a categoryData.xml file, replacing any previous data.
func (c *CategoryData) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return c.load(data)
}

func (c *CategoryData) load(data []byte) error {
	var doc xmlCategoryList
	if err := xml.Unmarshal(data, &doc); err != nil {
		return err
	}
	cats := make(map[string]map[int]bool, len(doc.Categories))
	for _, cat := range doc.Categories {
		set := make(map[int]bool, len(cat.IDs))
		for _, id := range cat.IDs {
			set[id] = true
		}
		cats[cat.Name] = set
	}
	c.mu.Lock()
	c.categories, c.loaded = cats, true
	c.mu.Unlock()
	return nil
}

// InCategory reports whether classID belongs to the named category.
func (c *CategoryData) InCategory(category string, classID int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	set, ok := c.categories[category]
	if !ok {
		return false
	}
	return set[classID]
}

type xmlCategoryList struct {
	XMLName    xml.Name      `xml:"list"`
	Categories []xmlCategory `xml:"category"`
}

type xmlCategory struct {
	Name string `xml:"name,attr"`
	IDs  []int  `xml:"id"`
}
