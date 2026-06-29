package client

func init() { addStubRegistrator(registerPartyMatchingStubs) }

// registerPartyMatchingStubs регистрирует стаб-обработчики поиска группы (High Five).
func registerPartyMatchingStubs(r *Registry) {
	// RequestPartyMatchConfig (0x7f): конфигурация поиска группы (создание/настройка).
	r.registerStub(StateInGame, 0x7f, "RequestPartyMatchConfig")
	// RequestPartyMatchList (0x80): список ожидающих группы (комнат поиска).
	r.registerStub(StateInGame, 0x80, "RequestPartyMatchList")
	// RequestPartyMatchDetail (0x81): детали объявления о поиске группы.
	r.registerStub(StateInGame, 0x81, "RequestPartyMatchDetail")
	// RequestOustFromPartyRoom (0xD0:0x09): исключить игрока из комнаты группы.
	r.registerMultiStub(StateInGame, 0x09, "RequestOustFromPartyRoom")
	// RequestDismissPartyRoom (0xD0:0x0a): распустить комнату группы (лидером).
	r.registerMultiStub(StateInGame, 0x0a, "RequestDismissPartyRoom")
	// RequestWithdrawPartyRoom (0xD0:0x0b): выйти из комнаты группы.
	r.registerMultiStub(StateInGame, 0x0b, "RequestWithdrawPartyRoom")
	// RequestExitPartyMatchingWaitingRoom (0xD0:0x25): выйти из комнаты ожидания.
	r.registerMultiStub(StateInGame, 0x25, "RequestExitPartyMatchingWaitingRoom")
	// RequestAskJoinPartyRoom (0xD0:0x2f): запросить вступление в комнату группы.
	r.registerMultiStub(StateInGame, 0x2f, "RequestAskJoinPartyRoom")
	// AnswerJoinPartyRoom (0xD0:0x30): ответ на приглашение в комнату группы.
	r.registerMultiStub(StateInGame, 0x30, "AnswerJoinPartyRoom")
	// RequestListPartyMatchingWaitingRoom (0xD0:0x31): список ожидающих в комнате.
	r.registerMultiStub(StateInGame, 0x31, "RequestListPartyMatchingWaitingRoom")
}
