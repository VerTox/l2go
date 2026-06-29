package client

import "testing"

// cb4.5: полнота слоя входящих пакетов — реестр распознаёт пакеты всех доменов.
// Проверяем суммарное число записей и точечно резолвим пакеты разных доменов
// (включая состояние Authed и мультипакеты 0xD0).

func TestRegistryEntryCount(t *testing.T) {
	r := buildRegistry()
	total := 0
	for _, m := range r.simple {
		total += len(m)
	}
	for _, m := range r.multi {
		total += len(m)
	}
	t.Logf("registry entries: %d", total)
	if total < 260 {
		t.Errorf("в реестре %d записей, ожидалось >= 260 (покрытие HF неполное)", total)
	}
}

func TestDomainStubsResolve(t *testing.T) {
	r := buildRegistry()
	cases := []struct {
		name   string
		state  ConnState
		opcode uint8
		sub    uint16
		want   string
	}{
		{"Say2", StateInGame, 0x49, 0, "Say2"},
		{"RequestJoinParty", StateInGame, 0x42, 0, "RequestJoinParty"},
		{"RequestStartPledgeWar", StateInGame, 0x03, 0, "RequestStartPledgeWar"},
		{"RequestSellItem", StateInGame, 0x37, 0, "RequestSellItem"},
		{"RequestMagicSkillUse", StateInGame, 0x39, 0, "RequestMagicSkillUse"},
		{"Mail/RequestPostItemList", StateInGame, 0xD0, 0x65, "RequestPostItemList"},
		{"Olympiad/MatchList", StateInGame, 0xD0, 0x2e, "RequestOlympiadMatchList"},
		{"Vehicle/ExGetOnAirShip", StateInGame, 0xD0, 0x36, "ExGetOnAirShip"},
		{"2ndPassword(Authed)", StateAuthed, 0xD0, 0x93, "RequestEx2ndPasswordCheck"},
		{"CharacterRestore(Authed)", StateAuthed, 0x7b, 0, "CharacterRestore"},
	}
	for _, c := range cases {
		e, ok := r.Resolve(c.state, c.opcode, c.sub)
		if !ok {
			t.Errorf("%s: не резолвится", c.name)
			continue
		}
		if e.Name != c.want {
			t.Errorf("%s: Name = %q, want %q", c.name, e.Name, c.want)
		}
	}
}

// GotoLobby (0xD0:0x36) в Authed и ExGetOnAirShip (0xD0:0x36) в InGame —
// проверяем, что состояние действительно разводит одинаковый sub-опкод.
func TestSameSubOpcodeDiffersByState(t *testing.T) {
	r := buildRegistry()
	authed, ok := r.Resolve(StateAuthed, 0xD0, 0x36)
	if !ok || authed.Name != "RequestGotoLobby" {
		t.Errorf("Authed 0xD0:0x36 = %q, want RequestGotoLobby", authed.Name)
	}
	inGame, ok := r.Resolve(StateInGame, 0xD0, 0x36)
	if !ok || inGame.Name != "ExGetOnAirShip" {
		t.Errorf("InGame 0xD0:0x36 = %q, want ExGetOnAirShip", inGame.Name)
	}
}
