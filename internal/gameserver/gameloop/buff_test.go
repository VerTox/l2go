package gameloop

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// windWalkXML: A2 self-buff, SPEED_UP, 3s, +50 runSpd.
const windWalkXML = `<list>
	<skill id="1204" levels="1" name="Wind Walk">
		<set name="abnormalTime" val="3" />
		<set name="abnormalType" val="SPEED_UP" />
		<set name="abnormalLvl" val="1" />
		<set name="hitTime" val="1000" />
		<set name="mpConsume1" val="10" />
		<set name="operateType" val="A2" />
		<set name="targetType" val="SELF" />
		<effects>
			<effect name="Buff">
				<add stat="runSpd" val="50" />
			</effect>
		</effects>
	</skill>
</list>`

// regenBuffXML: A2 self-buff HoT, 15s, TickHp 20 every 5s.
const regenBuffXML = `<list>
	<skill id="1204" levels="1" name="Regeneration">
		<set name="abnormalTime" val="15" />
		<set name="abnormalType" val="HP_RECOVER" />
		<set name="hitTime" val="1000" />
		<set name="mpConsume1" val="10" />
		<set name="operateType" val="A2" />
		<set name="targetType" val="SELF" />
		<effects>
			<effect name="TickHp">
				<param power="20" />
				<param ticks="5" />
			</effect>
		</effects>
	</skill>
</list>`

func loopWithBuffSkill(t *testing.T, xml string) (*GameLoop, *registry.PlayerWorldState) {
	t.Helper()
	gl, player := newTestLoopWithPlayer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "01200-01299.xml"), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	gl.SetSkillData(registry.NewSkillData([]string{dir}))
	player.KnownSkills = map[int32]int32{1204: 1}
	player.Character.MaxMP, player.Character.CurrentMP = 200, 200
	return gl, player
}

func TestBuff_CastAppliesStatMod(t *testing.T) {
	gl, player := loopWithBuffSkill(t, windWalkXML)

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1204})
	if player.Casting == nil {
		t.Fatal("buff cast did not begin")
	}
	(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)

	if player.Effects.Len() != 1 {
		t.Fatalf("Effects.Len = %d, want 1", player.Effects.Len())
	}
	// StatMods must now carry the buff's runSpd modifier.
	found := false
	for _, m := range player.Character.StatMods {
		if m.Stat == models.StatRunSpd && m.Op == "add" && m.Val == 50 {
			found = true
		}
	}
	if !found {
		t.Errorf("runSpd buff mod not in StatMods: %+v", player.Character.StatMods)
	}
}

func TestBuff_Expiry(t *testing.T) {
	gl, player := loopWithBuffSkill(t, windWalkXML)
	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1204})
	(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)
	if player.Effects.Len() != 1 {
		t.Fatal("buff not applied")
	}

	// Force expiry into the past and service buffs.
	player.Effects.Buffs()[0].ExpiresAt = time.Now().Add(-time.Second)
	gl.serviceBuffs()

	if player.Effects.Len() != 0 {
		t.Errorf("buff not expired: Len = %d", player.Effects.Len())
	}
	// StatMods back to base (no buff mods).
	for _, m := range player.Character.StatMods {
		if m.Stat == models.StatRunSpd {
			t.Errorf("runSpd mod lingered after expiry: %+v", player.Character.StatMods)
		}
	}
}

func TestBuff_ToggleOnOff(t *testing.T) {
	// Toggle variant of Wind Walk.
	xml := `<list><skill id="1204" levels="1" name="Toggle">
		<set name="operateType" val="T" /><set name="targetType" val="SELF" />
		<set name="abnormalType" val="NONE" />
		<effects><effect name="Buff"><add stat="pAtk" val="10" /></effect></effects>
	</skill></list>`
	gl, player := loopWithBuffSkill(t, xml)

	// First cast: toggle ON (instant, no cast bar for toggles).
	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1204})
	// Toggles still go through the cast pipeline; fire the hit to apply.
	if player.Casting != nil {
		(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)
	}
	if !player.Effects.HasSkill(1204) {
		t.Fatal("toggle should be active after first cast")
	}

	// Second cast: toggle OFF.
	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1204})
	if player.Effects.HasSkill(1204) {
		t.Error("toggle should be removed after recast")
	}
}

func TestBuff_HoTTick(t *testing.T) {
	gl, player := loopWithBuffSkill(t, regenBuffXML)
	player.Character.MaxHP, player.Character.CurrentHP = 500, 100

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1204})
	(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)
	if player.Effects.Len() != 1 {
		t.Fatal("HoT buff not applied")
	}

	// Force the tick due and service.
	player.Effects.Buffs()[0].NextTick = time.Now().Add(-time.Second)
	gl.serviceBuffs()

	if player.Character.CurrentHP != 120 {
		t.Errorf("HoT tick HP = %v, want 120 (100 + 20)", player.Character.CurrentHP)
	}
}
