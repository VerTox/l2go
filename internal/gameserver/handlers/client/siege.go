package client

func init() { addStubRegistrator(registerSiegeStubs) }

// registerSiegeStubs регистрирует стаб-обработчики пакетов осады (High Five).
func registerSiegeStubs(r *Registry) {
	// RequestSiegeInfo (0x58): запрос информации об осаде.
	r.registerStub(StateInGame, 0x58, "RequestSiegeInfo")
	// RequestBlock (0xa9): заблокировать игрока.
	r.registerStub(StateInGame, 0xa9, "RequestBlock")
	// RequestSiegeInfo2 (0xaa): информация об осаде (альтернативный опкод).
	r.registerStub(StateInGame, 0xaa, "RequestSiegeInfo2")
	// RequestSiegeAttackerList (0xab): запрос списка атакующих осаду.
	r.registerStub(StateInGame, 0xab, "RequestSiegeAttackerList")
	// RequestSiegeDefenderList (0xac): запрос списка защитников осады.
	r.registerStub(StateInGame, 0xac, "RequestSiegeDefenderList")
	// RequestJoinSiege (0xad): вступить в осаду.
	r.registerStub(StateInGame, 0xad, "RequestJoinSiege")
	// RequestConfirmSiegeWaitingList (0xae): подтвердить список ожидающих.
	r.registerStub(StateInGame, 0xae, "RequestConfirmSiegeWaitingList")
	// RequestSetCastleSiegeTime (0xaf): назначить время осады замка.
	r.registerStub(StateInGame, 0xaf, "RequestSetCastleSiegeTime")
	// RequestFortressSiegeInfo (0xD0:0x3f): запрос информации об осаде крепости.
	r.registerMultiStub(StateInGame, 0x3f, "RequestFortressSiegeInfo")
	// RequestFortressMapInfo (0xD0:0x48): запрос карты крепости.
	r.registerMultiStub(StateInGame, 0x48, "RequestFortressMapInfo")
	// RequestJoinDominionWar (0xD0:0x57): вступить в территориальную войну.
	r.registerMultiStub(StateInGame, 0x57, "RequestJoinDominionWar")
	// RequestDominionInfo (0xD0:0x58): запрос информации о территориальной войне.
	r.registerMultiStub(StateInGame, 0x58, "RequestDominionInfo")
}
