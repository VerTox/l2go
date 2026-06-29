package client

func init() { addStubRegistrator(registerPartyStubs) }

// registerPartyStubs регистрирует стаб-обработчики пакетов группы (High Five).
func registerPartyStubs(r *Registry) {
	// RequestJoinParty (0x42): пригласить игрока в группу. Для party-системы.
	r.registerStub(StateInGame, 0x42, "RequestJoinParty")
	// RequestAnswerJoinParty (0x43): ответ приглашённого на приглашение в группу.
	r.registerStub(StateInGame, 0x43, "RequestAnswerJoinParty")
	// RequestWithDrawalParty (0x44): добровольный выход игрока из группы.
	r.registerStub(StateInGame, 0x44, "RequestWithDrawalParty")
	// RequestOustPartyMember (0x45): исключить участника из группы (лидером).
	r.registerStub(StateInGame, 0x45, "RequestOustPartyMember")
	// RequestChangePartyLeader (0xD0:0x0c): сменить лидера группы.
	r.registerMultiStub(StateInGame, 0x0c, "RequestChangePartyLeader")
	// RequestPartyLootModification (0xD0:0x78): изменить тип распределения лута.
	r.registerMultiStub(StateInGame, 0x78, "RequestPartyLootModification")
	// AnswerPartyLootModification (0xD0:0x79): ответ на изменение типа лута.
	r.registerMultiStub(StateInGame, 0x79, "AnswerPartyLootModification")
}
