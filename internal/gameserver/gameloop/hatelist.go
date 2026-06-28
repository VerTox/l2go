package gameloop

// HateList tracks accumulated threat (hate) per attacker for an NPC.
type HateList struct {
	entries map[int32]int64 // charID -> total damage/hate
}

// NewHateList creates a new empty hate list.
func NewHateList() *HateList {
	return &HateList{
		entries: make(map[int32]int64),
	}
}

// AddHate adds hate for a character.
func (hl *HateList) AddHate(charID int32, amount int64) {
	hl.entries[charID] += amount
}

// GetTopAttacker returns the charID with the highest hate, or 0 if empty.
func (hl *HateList) GetTopAttacker() int32 {
	var topID int32
	var topHate int64
	for charID, hate := range hl.entries {
		if hate > topHate {
			topHate = hate
			topID = charID
		}
	}
	return topID
}

// GetAllAttackers returns all charIDs in the hate list.
func (hl *HateList) GetAllAttackers() []int32 {
	result := make([]int32, 0, len(hl.entries))
	for charID := range hl.entries {
		result = append(result, charID)
	}
	return result
}

// Clear resets the hate list.
func (hl *HateList) Clear() {
	hl.entries = make(map[int32]int64)
}

// IsEmpty returns true if no one has hate on this NPC.
func (hl *HateList) IsEmpty() bool {
	return len(hl.entries) == 0
}
