package client

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

// ConnState — состояние клиентского соединения. Один и тот же опкод в разных
// состояниях может означать разные пакеты (напр. 0xD0:0x36 = RequestGotoLobby
// в Authed и ExGetOnAirShip в InGame в хронике High Five), поэтому
// диспетчеризация учитывает состояние.
type ConnState uint8

const (
	// StateConnected — после хендшейка (ProtocolVersion/KeyPacket), до AuthLogin.
	StateConnected ConnState = iota
	// StateAuthed — после AuthLogin: экран выбора/создания персонажа.
	StateAuthed
	// StateInGame — после EnterWorld: игрок в мире.
	StateInGame
)

// multiPacketOpcode — префикс мультипакета (за ним идёт 2-байтный sub-опкод).
const multiPacketOpcode uint8 = 0xD0

// handlerFunc — единая сигнатура обработчика входящего пакета (method expression
// от *Handler или замыкание-стаб ей удовлетворяют).
type handlerFunc func(h *Handler, ctx context.Context, c *client.ClientConn, payload []byte) error

// packetEntry описывает один входящий пакет в реестре.
type packetEntry struct {
	Name       string     // имя пакета (для логов и читаемости реестра)
	Handle     handlerFunc // обработчик
	Transition *ConnState // если не nil — состояние после успешной обработки
	Fatal      bool       // true → ошибка обработчика рвёт соединение (как раньше)
}

// Registry — таблица опкод→обработчик с учётом состояния.
// simple: обычные однобайтные опкоды; multi: sub-опкоды мультипакета 0xD0.
type Registry struct {
	simple map[ConnState]map[uint8]packetEntry
	multi  map[ConnState]map[uint16]packetEntry
}

// Resolve возвращает запись для (state, opcode[, sub]) или ok=false, если опкод
// не зарегистрирован в данном состоянии. Для opcode==0xD0 используется sub.
func (r *Registry) Resolve(state ConnState, opcode uint8, sub uint16) (packetEntry, bool) {
	if opcode == multiPacketOpcode {
		if e, ok := r.multi[state][sub]; ok {
			return e, true
		}
		// Fallback: на экране выбора (StateAuthed) клиент шлёт gameplay-config
		// пакеты 0xD0 (RequestKeyMapping и др.), зарегистрированные в StateInGame.
		// Ищем их там. Прямые попадания в текущем состоянии имеют приоритет, поэтому
		// state-коллизии (0xD0:0x36 GotoLobby/ExGetOnAirShip) не ломаются.
		if state != StateInGame {
			if e, ok := r.multi[StateInGame][sub]; ok {
				return e, true
			}
		}
		return packetEntry{}, false
	}
	e, ok := r.simple[state][opcode]
	return e, ok
}

// parseSubOpcode читает 2-байтный sub-опкод (LE) мультипакета 0xD0 и возвращает
// оставшийся payload. Подтверждено реальными байтами клиента (GotoLobby = "3600" =
// sub 0x0036, тело пустое): sub-опкод — это readH (2 байта), как в L2J.
func parseSubOpcode(payload []byte) (uint16, []byte, bool) {
	if len(payload) < 2 {
		return 0, nil, false
	}
	sub := uint16(payload[0]) | uint16(payload[1])<<8
	return sub, payload[2:], true
}

// statePtr — хелпер для поля Transition.
func statePtr(s ConnState) *ConnState { return &s }

// noopStub — обработчик-заглушка: тихо игнорирует пакет (поведение прежних
// `continue`-веток). Доменные стабы (cb4.6..) логируют на уровне Warn.
func noopStub(name string) handlerFunc {
	return func(_ *Handler, ctx context.Context, _ *client.ClientConn, _ []byte) error {
		log.Ctx(ctx).Debug().Str("packet", name).Msg("packet ignored (stub)")
		return nil
	}
}

// warnStub — обработчик-заглушка доменного пакета: логирует факт получения
// (пока без обработки). Используется доменными стаб-файлами через registerStub.
func warnStub(name string) handlerFunc {
	return func(_ *Handler, ctx context.Context, _ *client.ClientConn, _ []byte) error {
		log.Ctx(ctx).Warn().Str("packet", name).Msg("received packet — not implemented")
		return nil
	}
}

// stubRegistrators — самораздача доменных стабов. Каждый доменный файл
// (handlers/client/<domain>.go) в своём init() вызывает addStubRegistrator,
// а buildRegistry прогоняет их все. Так домены не редактируют общий buildRegistry
// и могут разрабатываться независимо/параллельно.
var stubRegistrators []func(*Registry)

func addStubRegistrator(f func(*Registry)) { stubRegistrators = append(stubRegistrators, f) }

// registerStub регистрирует доменный стаб для обычного опкода в заданном
// состоянии. Паникует при коллизии (опкод уже занят) — чтобы дубль/конфликт
// всплыл сразу на тестах, а не молча перезаписал обработчик.
func (r *Registry) registerStub(state ConnState, opcode uint8, name string) {
	if e, exists := r.simple[state][opcode]; exists {
		panic(fmt.Sprintf("duplicate handler: state=%d opcode=0x%x (%s vs %s)", state, opcode, e.Name, name))
	}
	r.simple[state][opcode] = packetEntry{Name: name, Handle: warnStub(name)}
}

// registerMultiStub регистрирует доменный стаб для sub-опкода мультипакета 0xD0.
func (r *Registry) registerMultiStub(state ConnState, sub uint16, name string) {
	if e, exists := r.multi[state][sub]; exists {
		panic(fmt.Sprintf("duplicate handler: state=%d 0xD0:0x%x (%s vs %s)", state, sub, e.Name, name))
	}
	r.multi[state][sub] = packetEntry{Name: name, Handle: warnStub(name)}
}

// buildRegistry строит реестр входящих пакетов. На этапе фундамента (cb4.1)
// сюда перенесены только уже реализованные обработчики; доменные стабы
// добавляются отдельными задачами cb4.6..cb4.42.
func buildRegistry() *Registry {
	r := &Registry{
		simple: map[ConnState]map[uint8]packetEntry{
			StateConnected: {},
			StateAuthed:    {},
			StateInGame:    {},
		},
		multi: map[ConnState]map[uint16]packetEntry{
			StateConnected: {},
			StateAuthed:    {},
			StateInGame:    {},
		},
	}

	// --- StateConnected: ожидаем аутентификацию ---
	r.simple[StateConnected][0x2b] = packetEntry{
		Name: "AuthLogin", Handle: (*Handler).handleAuthLogin,
		Transition: statePtr(StateAuthed), Fatal: true,
	}

	// --- StateAuthed: экран выбора/создания персонажа ---
	r.simple[StateAuthed][0x12] = packetEntry{Name: "CharacterSelect", Handle: (*Handler).handleCharacterSelect, Fatal: true}
	r.simple[StateAuthed][0x13] = packetEntry{Name: "NewCharacter", Handle: (*Handler).handleNewCharacter, Fatal: true}
	r.simple[StateAuthed][0x0c] = packetEntry{Name: "CharacterCreate", Handle: (*Handler).handleCharacterCreate, Fatal: true}
	r.simple[StateAuthed][0x0d] = packetEntry{Name: "CharacterDelete", Handle: (*Handler).handleCharacterDelete, Fatal: true}
	r.simple[StateAuthed][0x11] = packetEntry{Name: "EnterWorld", Handle: (*Handler).handleEnterWorld, Transition: statePtr(StateInGame), Fatal: true}
	r.simple[StateAuthed][0x00] = packetEntry{Name: "Logout", Handle: (*Handler).handleLogout}
	r.simple[StateAuthed][0x57] = packetEntry{Name: "RequestRestart", Handle: (*Handler).handleRequestRestart}
	// 0xD0:0x36 — RequestGotoLobby (возврат в лобби после создания персонажа).
	r.multi[StateAuthed][0x36] = packetEntry{Name: "RequestGotoLobby", Handle: (*Handler).handleRequestGotoLobby}

	// --- StateInGame: игрок в мире ---
	r.simple[StateInGame][0x1f] = packetEntry{Name: "Action", Handle: (*Handler).handleAction}
	r.simple[StateInGame][0x48] = packetEntry{Name: "RequestTargetCancel", Handle: (*Handler).handleRequestTargetCancel}
	r.simple[StateInGame][0x0f] = packetEntry{Name: "MoveBackwardToLocation", Handle: (*Handler).handleMoveBackwardToLocation}
	r.simple[StateInGame][0x59] = packetEntry{Name: "ValidatePosition", Handle: (*Handler).handleValidatePosition}
	r.simple[StateInGame][0x47] = packetEntry{Name: "CannotMoveAnymore", Handle: (*Handler).handleCannotMoveAnymore}
	r.simple[StateInGame][0x52] = packetEntry{Name: "MoveWithDelta", Handle: noopStub("MoveWithDelta")}
	r.simple[StateInGame][0x56] = packetEntry{Name: "RequestActionUse", Handle: (*Handler).handleRequestActionUse}
	r.simple[StateInGame][0x00] = packetEntry{Name: "Logout", Handle: (*Handler).handleLogout}
	r.simple[StateInGame][0x57] = packetEntry{Name: "RequestRestart", Handle: (*Handler).handleRequestRestart}
	r.simple[StateInGame][0x14] = packetEntry{Name: "RequestItemList", Handle: (*Handler).handleRequestItemList}
	r.simple[StateInGame][0x19] = packetEntry{Name: "UseItem", Handle: (*Handler).handleUseItem}
	r.simple[StateInGame][0x16] = packetEntry{Name: "RequestUnEquipItem", Handle: (*Handler).handleRequestUnEquipItem}
	r.simple[StateInGame][0xa6] = packetEntry{Name: "RequestSkillCoolTime", Handle: noopStub("RequestSkillCoolTime")}

	// 0xD0 sub-опкоды в игре:
	r.multi[StateInGame][0x01] = packetEntry{Name: "RequestManorList", Handle: noopStub("RequestManorList")}
	r.multi[StateInGame][0x0d] = packetEntry{Name: "RequestAutoSoulShot", Handle: (*Handler).handleRequestAutoSoulShot}
	r.multi[StateInGame][0x21] = packetEntry{Name: "RequestKeyMapping", Handle: (*Handler).handleRequestKeyMapping}
	r.multi[StateInGame][0x22] = packetEntry{Name: "RequestSaveKeyMapping", Handle: (*Handler).handleRequestSaveKeyMapping}
	// Баг cb4.4: 0xD0:0x24 — это RequestSaveInventoryOrder, а не «Unknown».
	r.multi[StateInGame][0x24] = packetEntry{Name: "RequestSaveInventoryOrder", Handle: noopStub("RequestSaveInventoryOrder")}

	// Доменные стабы (cb4.6..cb4.42) самораздаются через init() + addStubRegistrator.
	for _, register := range stubRegistrators {
		register(r)
	}

	return r
}
