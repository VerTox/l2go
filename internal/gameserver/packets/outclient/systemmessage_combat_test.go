package outclient

import (
	"testing"

	"github.com/VerTox/l2go/pkg/l2pkt"
)

// TestSystemMessage_PlayerDamage verifies the wire encoding of the player->mob
// damage message (C1_DONE_S3_DAMAGE_TO_C2) byte-for-byte against L2J HF: opcode
// 0x62, msgId, param count, then each param as [type(D), value]. Types: PLAYER_NAME
// (12, UTF-16 string), NPC_NAME (2, 1000000+templateId as D), INT (1, D).
func TestSystemMessage_PlayerDamage(t *testing.T) {
	const templateID int32 = 100
	pkt := NewSystemMessage(SysMsgC1DoneS3DamageToC2).
		AddPlayerName("Hero").
		AddNpcName(templateID).
		AddInt(50).
		Build()

	r := l2pkt.NewReader(pkt)
	if op, _ := r.ReadC(); op != 0x62 {
		t.Fatalf("opcode = 0x%02x, want 0x62", op)
	}
	if id, _ := r.ReadD(); id != SysMsgC1DoneS3DamageToC2 {
		t.Fatalf("msgId = %d, want %d", id, SysMsgC1DoneS3DamageToC2)
	}
	if n, _ := r.ReadD(); n != 3 {
		t.Fatalf("param count = %d, want 3", n)
	}

	// param 1: PLAYER_NAME
	if ty, _ := r.ReadD(); ty != smParamPlayerName {
		t.Errorf("param1 type = %d, want %d (PLAYER_NAME)", ty, smParamPlayerName)
	}
	if s, _ := r.ReadS(); s != "Hero" {
		t.Errorf("param1 value = %q, want Hero", s)
	}
	// param 2: NPC_NAME encoded as 1000000+templateId
	if ty, _ := r.ReadD(); ty != smParamNpcName {
		t.Errorf("param2 type = %d, want %d (NPC_NAME)", ty, smParamNpcName)
	}
	if v, _ := r.ReadD(); v != 1000000+templateID {
		t.Errorf("param2 value = %d, want %d", v, 1000000+templateID)
	}
	// param 3: INT damage
	if ty, _ := r.ReadD(); ty != smParamInt {
		t.Errorf("param3 type = %d, want %d (INT)", ty, smParamInt)
	}
	if v, _ := r.ReadD(); v != 50 {
		t.Errorf("param3 value = %d, want 50", v)
	}
}

// TestSystemMessage_VictimDamage verifies the mob->player damage message
// (C1_RECEIVED_DAMAGE_OF_S3_FROM_C2): C1 is the victim name as plain TEXT (not
// PLAYER_NAME — an L2J quirk), then NPC_NAME, then INT.
func TestSystemMessage_VictimDamage(t *testing.T) {
	const templateID int32 = 20001
	pkt := NewSystemMessage(SysMsgC1ReceivedDamageS3FromC2).
		AddString("Victim").
		AddNpcName(templateID).
		AddInt(123).
		Build()

	r := l2pkt.NewReader(pkt)
	_, _ = r.ReadC()
	if id, _ := r.ReadD(); id != SysMsgC1ReceivedDamageS3FromC2 {
		t.Fatalf("msgId = %d, want %d", id, SysMsgC1ReceivedDamageS3FromC2)
	}
	if n, _ := r.ReadD(); n != 3 {
		t.Fatalf("param count = %d, want 3", n)
	}
	if ty, _ := r.ReadD(); ty != smParamText {
		t.Errorf("param1 type = %d, want %d (TEXT)", ty, smParamText)
	}
	if s, _ := r.ReadS(); s != "Victim" {
		t.Errorf("param1 value = %q, want Victim", s)
	}
	if ty, _ := r.ReadD(); ty != smParamNpcName {
		t.Errorf("param2 type = %d, want NPC_NAME", ty)
	}
	if v, _ := r.ReadD(); v != 1000000+templateID {
		t.Errorf("param2 value = %d, want %d", v, 1000000+templateID)
	}
	if ty, _ := r.ReadD(); ty != smParamInt {
		t.Errorf("param3 type = %d, want INT", ty)
	}
	if v, _ := r.ReadD(); v != 123 {
		t.Errorf("param3 value = %d, want 123", v)
	}
}

// TestSystemMessage_MissAndCrit verifies the single-param miss/crit messages carry
// just the attacker's PLAYER_NAME.
func TestSystemMessage_MissAndCrit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		msgID int32
	}{
		{"miss", SysMsgC1AttackWentAstray},
		{"crit", SysMsgC1HadCriticalHit},
	} {
		pkt := NewSystemMessage(tc.msgID).AddPlayerName("Hero").Build()
		r := l2pkt.NewReader(pkt)
		_, _ = r.ReadC()
		if id, _ := r.ReadD(); id != tc.msgID {
			t.Fatalf("%s msgId = %d, want %d", tc.name, id, tc.msgID)
		}
		if n, _ := r.ReadD(); n != 1 {
			t.Fatalf("%s param count = %d, want 1", tc.name, n)
		}
		if ty, _ := r.ReadD(); ty != smParamPlayerName {
			t.Errorf("%s param type = %d, want PLAYER_NAME", tc.name, ty)
		}
		if s, _ := r.ReadS(); s != "Hero" {
			t.Errorf("%s param value = %q, want Hero", tc.name, s)
		}
	}
}
