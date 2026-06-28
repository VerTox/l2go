package client

import "testing"

// Фундамент cb4.1: state-aware реестр диспетчеризации входящих пакетов.
// Тесты фиксируют контракт реестра, разбор sub-опкода 0xD0 и переходы состояний.

func TestResolveExistingSimpleOpcode(t *testing.T) {
	r := buildRegistry()

	// AuthLogin (0x2b) принимается в состоянии Connected.
	e, ok := r.Resolve(StateConnected, 0x2b, 0)
	if !ok {
		t.Fatal("AuthLogin (0x2b) должен резолвиться в StateConnected")
	}
	if e.Name != "AuthLogin" {
		t.Errorf("Name = %q, want AuthLogin", e.Name)
	}
}

func TestAuthLoginTransitionsToAuthed(t *testing.T) {
	r := buildRegistry()
	e, ok := r.Resolve(StateConnected, 0x2b, 0)
	if !ok {
		t.Fatal("AuthLogin должен резолвиться")
	}
	if e.Transition == nil || *e.Transition != StateAuthed {
		t.Errorf("AuthLogin должен переводить в StateAuthed")
	}
}

func TestEnterWorldTransitionsToInGame(t *testing.T) {
	r := buildRegistry()
	e, ok := r.Resolve(StateAuthed, 0x11, 0)
	if !ok {
		t.Fatal("EnterWorld (0x11) должен резолвиться в StateAuthed")
	}
	if e.Transition == nil || *e.Transition != StateInGame {
		t.Errorf("EnterWorld должен переводить в StateInGame")
	}
}

func TestInGameOpcodeNotResolvedInConnected(t *testing.T) {
	r := buildRegistry()
	// Action (0x1f) — игровой пакет, не должен резолвиться до входа в мир.
	if _, ok := r.Resolve(StateConnected, 0x1f, 0); ok {
		t.Error("Action (0x1f) не должен резолвиться в StateConnected")
	}
	if _, ok := r.Resolve(StateInGame, 0x1f, 0); !ok {
		t.Error("Action (0x1f) должен резолвиться в StateInGame")
	}
}

func TestResolveMultiPacketGotoLobby(t *testing.T) {
	r := buildRegistry()
	// 0xD0:0x36 = RequestGotoLobby в состоянии Authed.
	e, ok := r.Resolve(StateAuthed, 0xD0, 0x36)
	if !ok {
		t.Fatal("0xD0:0x36 должен резолвиться в StateAuthed")
	}
	if e.Name != "RequestGotoLobby" {
		t.Errorf("Name = %q, want RequestGotoLobby", e.Name)
	}
}

// Баг cb4.2: sub-опкод 0xD0 читается как 2-байтный uint16 LE, payload сдвигается на 2.
func TestParseSubOpcodeTwoBytesLittleEndian(t *testing.T) {
	sub, rest, ok := parseSubOpcode([]byte{0x36, 0x00, 0xAA, 0xBB})
	if !ok {
		t.Fatal("parseSubOpcode должен успешно прочитать 2 байта")
	}
	if sub != 0x0036 {
		t.Errorf("sub = %#x, want 0x0036", sub)
	}
	if len(rest) != 2 || rest[0] != 0xAA || rest[1] != 0xBB {
		t.Errorf("rest = % x, want AA BB (payload после 2-байтного sub)", rest)
	}
}

func TestParseSubOpcodeTooShort(t *testing.T) {
	if _, _, ok := parseSubOpcode([]byte{0x36}); ok {
		t.Error("parseSubOpcode должен вернуть false при payload < 2 байт")
	}
}

// Баг cb4.3: 0xD0:0x38 — это MoveToLocationAirShip, а не RequestGotoLobby.
func TestD038NotGotoLobby(t *testing.T) {
	r := buildRegistry()
	if e, ok := r.Resolve(StateAuthed, 0xD0, 0x38); ok && e.Name == "RequestGotoLobby" {
		t.Error("0xD0:0x38 не должен маппиться на RequestGotoLobby")
	}
	if e, ok := r.Resolve(StateInGame, 0xD0, 0x38); ok && e.Name == "RequestGotoLobby" {
		t.Error("0xD0:0x38 не должен маппиться на RequestGotoLobby")
	}
}

// Баг cb4.4: 0xD0:0x24 — это RequestSaveInventoryOrder, а не «Unknown».
func TestD024IsSaveInventoryOrder(t *testing.T) {
	r := buildRegistry()
	e, ok := r.Resolve(StateInGame, 0xD0, 0x24)
	if !ok {
		t.Fatal("0xD0:0x24 должен резолвиться в StateInGame")
	}
	if e.Name != "RequestSaveInventoryOrder" {
		t.Errorf("Name = %q, want RequestSaveInventoryOrder", e.Name)
	}
}

func TestUnknownOpcodeDoesNotResolve(t *testing.T) {
	r := buildRegistry()
	if _, ok := r.Resolve(StateInGame, 0xEE, 0); ok {
		t.Error("неизвестный опкод 0xEE не должен резолвиться")
	}
}
