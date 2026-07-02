package registry

import (
	"encoding/xml"
	"testing"
)

// parseSingleItem is a test helper that unmarshals an inline <list> XML fixture
// and converts the first item into an ItemTemplate.
func parseSingleItem(t *testing.T, doc string) *ItemTemplate {
	t.Helper()
	var list xmlItemList
	if err := xml.Unmarshal([]byte(doc), &list); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	if len(list.Items) == 0 {
		t.Fatalf("fixture contained no items")
	}
	r := NewItemTemplateRegistry()
	return r.convertXMLItem(list.Items[0])
}

func TestConvertXMLItem_UseItemFields(t *testing.T) {
	const doc = `<list>
		<item id="1463" type="EtcItem" name="Soulshot (C-Grade)">
			<set name="default_action" val="SOULSHOT" />
			<set name="immediate_effect" val="true" />
			<set name="is_oly_restricted" val="true" />
			<set name="handler" val="SoulShots" />
			<set name="item_skill" val="2150-1" />
			<set name="reuse_delay" val="500" />
			<set name="shared_reuse_group" val="7" />
		</item>
	</list>`

	tmpl := parseSingleItem(t, doc)

	if tmpl.Handler != "SoulShots" {
		t.Errorf("Handler = %q, want SoulShots", tmpl.Handler)
	}
	if tmpl.DefaultAction != "SOULSHOT" {
		t.Errorf("DefaultAction = %q, want SOULSHOT", tmpl.DefaultAction)
	}
	if !tmpl.ImmediateEffect {
		t.Error("ImmediateEffect = false, want true")
	}
	if !tmpl.IsOlyRestricted {
		t.Error("IsOlyRestricted = false, want true")
	}
	if tmpl.ReuseDelay != 500 {
		t.Errorf("ReuseDelay = %d, want 500", tmpl.ReuseDelay)
	}
	if tmpl.SharedReuseGroup != 7 {
		t.Errorf("SharedReuseGroup = %d, want 7", tmpl.SharedReuseGroup)
	}
	if len(tmpl.ItemSkills) != 1 || tmpl.ItemSkills[0] != (ItemSkill{ID: 2150, Level: 1}) {
		t.Errorf("ItemSkills = %+v, want [{2150 1}]", tmpl.ItemSkills)
	}
}

func TestConvertXMLItem_Defaults(t *testing.T) {
	const doc = `<list>
		<item id="99999" type="EtcItem" name="Plain Item">
			<set name="weight" val="1" />
		</item>
	</list>`

	tmpl := parseSingleItem(t, doc)

	if tmpl.Handler != "" {
		t.Errorf("Handler = %q, want empty", tmpl.Handler)
	}
	if tmpl.DefaultAction != "" {
		t.Errorf("DefaultAction = %q, want empty", tmpl.DefaultAction)
	}
	if tmpl.ItemSkills != nil {
		t.Errorf("ItemSkills = %+v, want nil", tmpl.ItemSkills)
	}
	if tmpl.ReuseDelay != 0 || tmpl.SharedReuseGroup != 0 {
		t.Errorf("reuse fields non-zero: %d/%d", tmpl.ReuseDelay, tmpl.SharedReuseGroup)
	}
	if tmpl.ImmediateEffect || tmpl.IsOlyRestricted || tmpl.QuestItem {
		t.Error("bool defaults should be false")
	}
}

func TestParseItemSkills(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []ItemSkill
	}{
		{"empty", "", nil},
		{"single", "2150-1", []ItemSkill{{2150, 1}}},
		{"multiple", "21006-1;21007-2", []ItemSkill{{21006, 1}, {21007, 2}}},
		{"skip zero id", "0-1;2004-1", []ItemSkill{{2004, 1}}},
		{"skip zero level", "2004-0;2005-3", []ItemSkill{{2005, 3}}},
		{"malformed skipped", "abc;2006-1", []ItemSkill{{2006, 1}}},
		{"trailing semicolon", "2007-1;", []ItemSkill{{2007, 1}}},
		{"all invalid", "0-0;bad", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseItemSkills(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("parseItemSkills(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseItemSkills(%q)[%d] = %+v, want %+v", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestComputeType2_QuestItem(t *testing.T) {
	const questDoc = `<list>
		<item id="1836" type="EtcItem" name="Order">
			<set name="is_questitem" val="true" />
		</item>
	</list>`
	if tmpl := parseSingleItem(t, questDoc); tmpl.Type2 != ItemType2Quest {
		t.Errorf("quest item Type2 = %d, want %d (QUEST)", tmpl.Type2, ItemType2Quest)
	}

	const adenaDoc = `<list>
		<item id="57" type="EtcItem" name="Adena">
			<set name="is_stackable" val="true" />
		</item>
	</list>`
	if tmpl := parseSingleItem(t, adenaDoc); tmpl.Type2 != ItemType2Money {
		t.Errorf("adena Type2 = %d, want %d (MONEY)", tmpl.Type2, ItemType2Money)
	}

	const otherDoc = `<list>
		<item id="1463" type="EtcItem" name="Soulshot">
			<set name="handler" val="SoulShots" />
		</item>
	</list>`
	if tmpl := parseSingleItem(t, otherDoc); tmpl.Type2 != ItemType2Other {
		t.Errorf("etc item Type2 = %d, want %d (OTHER)", tmpl.Type2, ItemType2Other)
	}
}
