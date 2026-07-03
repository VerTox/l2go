package registry

import (
	"encoding/xml"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/data"
)

// Reward exp/sp must come from the datapack <acquire> element: exp = level²×expRate,
// sp = raw sp. NPCs without <acquire> yield 0. (l2go-ejz)
func TestConvertXMLNpc_AcquireRewards(t *testing.T) {
	const doc = `<list>
		<npc id="22816" level="81" type="L2Monster" name="Maluk Maiden">
			<acquire expRate="3.84310575" sp="2792" />
		</npc>
		<npc id="1" level="10" type="L2Npc" name="Guard"></npc>
	</list>`

	var list xmlNpcList
	if err := xml.Unmarshal([]byte(doc), &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.NPCs) != 2 {
		t.Fatalf("parsed %d npcs, want 2", len(list.NPCs))
	}

	maluk := convertXMLNpc(list.NPCs[0])
	wantExp := data.CalcNPCBaseExp(81, 3.84310575)
	if maluk.RewardExp != wantExp {
		t.Errorf("RewardExp = %d, want %d (level²×expRate)", maluk.RewardExp, wantExp)
	}
	if maluk.RewardSp != 2792 {
		t.Errorf("RewardSp = %d, want 2792", maluk.RewardSp)
	}

	// No <acquire> → no reward.
	guard := convertXMLNpc(list.NPCs[1])
	if guard.RewardExp != 0 || guard.RewardSp != 0 {
		t.Errorf("guard rewards = (%d, %d), want (0, 0)", guard.RewardExp, guard.RewardSp)
	}
}
