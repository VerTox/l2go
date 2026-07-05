package gameloop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/VerTox/l2go/internal/gameserver/models"
	"github.com/VerTox/l2go/internal/gameserver/registry"
)

// heal1011XML is the datapack Heal skill (tabled, 18 levels). Level 1: mpConsume1=2,
// mpConsume2=8, reuseDelay=3000, Heal power=50, targetType TARGET (beneficial).
const heal1011XML = `<list>
	<skill id="1011" levels="18" name="Heal">
		<table name="#healPower"> 50 58 67 83 95 107 121 135 151 176 185 195 224 234 245 278 289 301 </table>
		<table name="#mpConsume1"> 2 3 3 4 4 5 5 6 6 7 7 8 9 9 9 10 10 11 </table>
		<table name="#mpConsume2"> 8 10 11 13 15 17 19 21 24 26 28 29 33 35 35 38 40 41 </table>
		<set name="hitTime" val="2000" />
		<set name="isMagic" val="1" />
		<set name="mpConsume1" val="#mpConsume1" />
		<set name="mpConsume2" val="#mpConsume2" />
		<set name="operateType" val="A1" />
		<set name="reuseDelay" val="3000" />
		<set name="targetType" val="TARGET" />
		<effects>
			<effect name="Heal">
				<param power="#healPower" />
			</effect>
		</effects>
	</skill>
</list>`

func loopWithHealSkill(t *testing.T) (*GameLoop, *registry.PlayerWorldState) {
	t.Helper()
	gl, player := newTestLoopWithPlayer(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "01000-01099.xml"), []byte(heal1011XML), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	gl.SetSkillData(registry.NewSkillData([]string{dir}))

	player.KnownSkills = map[int32]int32{1011: 1}
	return gl, player
}

func TestCast_HealSelf_FullChain(t *testing.T) {
	gl, player := loopWithHealSkill(t)
	c := player.Character
	c.MaxHP, c.CurrentHP = 500, 100
	c.MaxMP, c.CurrentMP = 200, 100

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1011})

	// beginCast: mpConsume1 (=2) spent, casting armed, self target.
	if player.Casting == nil {
		t.Fatal("Casting not set after beginCast")
	}
	if c.CurrentMP != 98 {
		t.Errorf("MP after beginCast = %v, want 98 (100-2)", c.CurrentMP)
	}
	if player.Casting.TargetID != 7 {
		t.Errorf("cast target = %d, want self (7)", player.Casting.TargetID)
	}
	castID := player.Casting.ID

	// Fire the scheduled hit.
	(&CastHitEvent{CharID: 7, CastID: castID}).Execute(gl)

	if player.Casting != nil {
		t.Error("Casting not cleared after hit")
	}
	if c.CurrentMP != 90 {
		t.Errorf("MP after hit = %v, want 90 (98-8 mpConsume2)", c.CurrentMP)
	}
	if c.CurrentHP != 150 {
		t.Errorf("HP after heal = %v, want 150 (100+50 power)", c.CurrentHP)
	}
	if !gl.isSkillOnReuse(7, 1011) {
		t.Error("skill reuse not armed after cast")
	}
}

func TestCast_NotEnoughMP(t *testing.T) {
	gl, player := loopWithHealSkill(t)
	c := player.Character
	c.MaxMP, c.CurrentMP = 200, 1 // below mpConsume1 (2)

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1011})

	if player.Casting != nil {
		t.Error("cast should fail with insufficient MP")
	}
	if c.CurrentMP != 1 {
		t.Errorf("MP = %v, want unchanged 1", c.CurrentMP)
	}
}

func TestCast_UnknownSkillIgnored(t *testing.T) {
	gl, player := loopWithHealSkill(t)
	player.KnownSkills = map[int32]int32{} // knows nothing

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1011})

	if player.Casting != nil {
		t.Error("cast of unknown skill should be ignored")
	}
}

func TestCast_ReuseBlocksRecast(t *testing.T) {
	gl, player := loopWithHealSkill(t)
	c := player.Character
	c.MaxHP, c.CurrentHP = 500, 100
	c.MaxMP, c.CurrentMP = 200, 100

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1011})
	(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)
	mpAfterFirst := c.CurrentMP

	// Immediate recast: blocked by reuse — no MP spent, no cast.
	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1011})
	if player.Casting != nil {
		t.Error("recast during cooldown should be blocked")
	}
	if c.CurrentMP != mpAfterFirst {
		t.Errorf("MP changed on blocked recast: %v != %v", c.CurrentMP, mpAfterFirst)
	}
}

func TestCalcMagicDamage(t *testing.T) {
	// (91 * sqrt(100) / 50) * 10 = (91*10/50)*10 = 18.2*10 = 182.
	if got := calcMagicDamage(100, 50, 10); got != 182 {
		t.Errorf("calcMagicDamage(100,50,10) = %d, want 182", got)
	}
	// mDef floored to 1; damage floored to >=1.
	if got := calcMagicDamage(1, 0, 0); got != 1 {
		t.Errorf("calcMagicDamage min = %d, want 1", got)
	}
}

const nuke9999XML = `<list>
	<skill id="9999" levels="1" name="Test Nuke">
		<set name="castRange" val="600" />
		<set name="hitTime" val="1500" />
		<set name="isMagic" val="1" />
		<set name="mpConsume1" val="5" />
		<set name="operateType" val="A1" />
		<set name="reuseDelay" val="2000" />
		<set name="targetType" val="ENEMY" />
		<effects>
			<effect name="MagicalAttack">
				<param power="40" />
			</effect>
		</effects>
	</skill>
</list>`

func TestCast_MagicDamageOnNPC(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "09900-09999.xml"), []byte(nuke9999XML), 0o644); err != nil {
		t.Fatal(err)
	}
	gl.SetSkillData(registry.NewSkillData([]string{dir}))
	player.KnownSkills = map[int32]int32{9999: 1}
	player.Character.MaxMP, player.Character.CurrentMP = 200, 200

	npc := addAttackableNPC(gl, 1000, models.Position{X: 0, Y: 0, Z: 0})
	npc.Template.MDef = 30
	npc.CurrentHP = 500
	player.TargetID = npc.ObjectID // targeting the mob

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 9999})
	if player.Casting == nil {
		t.Fatal("nuke did not begin casting (target/MP?)")
	}
	(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)

	if npc.CurrentHP >= 500 {
		t.Errorf("NPC HP = %v, want < 500 (took magic damage)", npc.CurrentHP)
	}
}

func TestCast_MovementAbortsCast(t *testing.T) {
	gl, player := loopWithHealSkill(t)
	c := player.Character
	c.MaxHP, c.CurrentHP = 500, 100
	c.MaxMP, c.CurrentMP = 200, 100

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1011})
	if player.Casting == nil {
		t.Fatal("cast did not begin")
	}
	castID := player.Casting.ID

	// Player moves → cast aborts.
	gl.handleMoveToLocation(CmdMoveToLocation{CharID: 7})
	if player.Casting != nil {
		t.Error("cast should be aborted on movement")
	}

	// The scheduled hit for the aborted cast must be a no-op (no heal).
	(&CastHitEvent{CharID: 7, CastID: castID}).Execute(gl)
	if c.CurrentHP != 100 {
		t.Errorf("aborted cast still healed: HP=%v, want 100", c.CurrentHP)
	}
}

func TestCast_MPCheckedUpfront(t *testing.T) {
	// heal 1011: mpConsume1=2, mpConsume2=8 (total 10). MP=5 covers the initial part
	// but not the full cost — the cast must be rejected up front (not fizzle at hit).
	gl, player := loopWithHealSkill(t)
	player.Character.MaxMP, player.Character.CurrentMP = 200, 5

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 1011})
	if player.Casting != nil {
		t.Error("cast should be rejected when MP < mpConsume1+mpConsume2")
	}
	if player.Character.CurrentMP != 5 {
		t.Errorf("MP = %v, want unchanged 5 (no partial consume)", player.Character.CurrentMP)
	}
}

// TestCast_OutOfRangeApproaches: an out-of-range cast no longer just fails — it
// starts a server-driven run toward the target (l2go-bdb), leaving the cast itself
// not yet begun.
func TestCast_OutOfRangeApproaches(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "09900-09999.xml"), []byte(nuke9999XML), 0o644); err != nil {
		t.Fatal(err)
	}
	gl.SetSkillData(registry.NewSkillData([]string{dir}))
	player.KnownSkills = map[int32]int32{9999: 1}
	player.Character.CurrentMP = 200

	npc := addAttackableNPC(gl, 1000, models.Position{X: 5000, Y: 0, Z: 0}) // beyond castRange 600
	player.TargetID = npc.ObjectID

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 9999})

	if player.Casting != nil {
		t.Error("cast must not begin while out of range — it should approach first")
	}
	if _, ok := gl.castPending[7]; !ok {
		t.Error("out-of-range cast should start an approach (castPending set)")
	}
	if st := gl.aiState[7]; st == nil || st.Intention != IntentionCast {
		t.Errorf("intention should be IntentionCast during approach, got %+v", st)
	}
	if !player.IsMoving {
		t.Error("caster should be moving toward the target")
	}
}

// TestCast_ApproachThenCastsOnArrival: once the caster is within range, the approach
// heartbeat begins the real cast and clears the pending approach.
func TestCast_ApproachThenCastsOnArrival(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "09900-09999.xml"), []byte(nuke9999XML), 0o644); err != nil {
		t.Fatal(err)
	}
	gl.SetSkillData(registry.NewSkillData([]string{dir}))
	player.KnownSkills = map[int32]int32{9999: 1}
	player.Character.CurrentMP = 200

	npc := addAttackableNPC(gl, 1000, models.Position{X: 5000, Y: 0, Z: 0})
	player.TargetID = npc.ObjectID

	cmd := CmdCastRequest{CasterCharID: 7, SkillID: 9999}
	gl.handleCastRequest(cmd) // starts the approach
	if _, ok := gl.castPending[7]; !ok {
		t.Fatal("approach not started")
	}

	// Simulate arrival within cast range (500 < 600) and fire the heartbeat.
	player.Position = models.Position{X: 4500, Y: 0, Z: 0}
	player.IsMoving = false
	(&CastApproachEvent{CharID: 7, TargetObjectID: npc.ObjectID, Cmd: cmd}).Execute(gl)

	if player.Casting == nil {
		t.Error("cast should begin once the caster arrives in range")
	}
	if _, ok := gl.castPending[7]; ok {
		t.Error("castPending should be cleared when the cast begins")
	}
}

// TestCast_GroundMoveCancelsApproach: a deliberate ground move aborts an in-flight
// cast approach (mirrors the interact-approach cancel).
func TestCast_GroundMoveCancelsApproach(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "09900-09999.xml"), []byte(nuke9999XML), 0o644); err != nil {
		t.Fatal(err)
	}
	gl.SetSkillData(registry.NewSkillData([]string{dir}))
	player.KnownSkills = map[int32]int32{9999: 1}
	player.Character.CurrentMP = 200
	npc := addAttackableNPC(gl, 1000, models.Position{X: 5000, Y: 0, Z: 0})
	player.TargetID = npc.ObjectID

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 9999})
	if _, ok := gl.castPending[7]; !ok {
		t.Fatal("approach not started")
	}

	gl.handleMoveToLocation(CmdMoveToLocation{CharID: 7})

	if _, ok := gl.castPending[7]; ok {
		t.Error("ground move should cancel the pending cast approach")
	}
}

func TestCalcPhysSkillDamage(t *testing.T) {
	// (76 * (pAtk + power)) / pDef = (76 * (100+50)) / 50 = 11400/50 = 228.
	if got := calcPhysSkillDamage(100, 50, 50); got != 228 {
		t.Errorf("calcPhysSkillDamage(100,50,50) = %d, want 228", got)
	}
	if got := calcPhysSkillDamage(0, 0, 0); got != 1 {
		t.Errorf("min damage = %d, want 1", got)
	}
}

const physSkillXML = `<list>
	<skill id="9998" levels="1" name="Power Strike">
		<set name="castRange" val="40" /><set name="hitTime" val="1000" />
		<set name="mpConsume1" val="3" /><set name="operateType" val="A1" />
		<set name="targetType" val="ENEMY" />
		<effects><effect name="PhysicalAttack"><param power="30" /></effect></effects>
	</skill>
</list>`

func TestCast_PhysicalDamageOnNPC(t *testing.T) {
	gl, player := newTestLoopWithPlayer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "09900-09999.xml"), []byte(physSkillXML), 0o644); err != nil {
		t.Fatal(err)
	}
	gl.SetSkillData(registry.NewSkillData([]string{dir}))
	player.KnownSkills = map[int32]int32{9998: 1}
	player.Character.CurrentMP = 200

	npc := addAttackableNPC(gl, 1000, models.Position{X: 0, Y: 0, Z: 0})
	npc.Template.PDef = 40
	npc.CurrentHP = 500
	player.TargetID = npc.ObjectID

	gl.handleCastRequest(CmdCastRequest{CasterCharID: 7, SkillID: 9998})
	if player.Casting == nil {
		t.Fatal("physical skill did not begin casting")
	}
	(&CastHitEvent{CharID: 7, CastID: player.Casting.ID}).Execute(gl)
	if npc.CurrentHP >= 500 {
		t.Errorf("NPC HP = %v, want < 500 (took physical skill damage)", npc.CurrentHP)
	}
}
