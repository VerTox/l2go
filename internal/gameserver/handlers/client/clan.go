package client

func init() { addStubRegistrator(registerClanStubs) }

// registerClanStubs регистрирует стаб-обработчики пакетов клана (High Five).
func registerClanStubs(r *Registry) {
	// RequestStartPledgeWar (0x03): объявить войну другому клану.
	r.registerStub(StateInGame, 0x03, "RequestStartPledgeWar")
	// RequestReplyStartPledgeWar (0x04): ответ на объявление клановой войны.
	r.registerStub(StateInGame, 0x04, "RequestReplyStartPledgeWar")
	// RequestStopPledgeWar (0x05): остановить клановую войну.
	r.registerStub(StateInGame, 0x05, "RequestStopPledgeWar")
	// RequestReplyStopPledgeWar (0x06): ответ на прекращение клановой войны.
	r.registerStub(StateInGame, 0x06, "RequestReplyStopPledgeWar")
	// RequestSurrenderPledgeWar (0x07): капитулировать в клановой войне.
	r.registerStub(StateInGame, 0x07, "RequestSurrenderPledgeWar")
	// RequestReplySurrenderPledgeWar (0x08): ответ на капитуляцию в войне.
	r.registerStub(StateInGame, 0x08, "RequestReplySurrenderPledgeWar")
	// RequestSetPledgeCrest (0x09): установить герб клана.
	r.registerStub(StateInGame, 0x09, "RequestSetPledgeCrest")
	// RequestGiveNickName (0x0b): дать титул члену клана.
	r.registerStub(StateInGame, 0x0b, "RequestGiveNickName")
	// RequestJoinPledge (0x26): принять игрока в клан.
	r.registerStub(StateInGame, 0x26, "RequestJoinPledge")
	// RequestAnswerJoinPledge (0x27): ответ на приглашение в клан.
	r.registerStub(StateInGame, 0x27, "RequestAnswerJoinPledge")
	// RequestWithdrawalPledge (0x28): добровольно выйти из клана.
	r.registerStub(StateInGame, 0x28, "RequestWithdrawalPledge")
	// RequestOustPledgeMember (0x29): исключить члена из клана.
	r.registerStub(StateInGame, 0x29, "RequestOustPledgeMember")
	// RequestPledgeMemberList (0x4d): запрос списка членов клана.
	r.registerStub(StateInGame, 0x4d, "RequestPledgeMemberList")
	// RequestPledgeInfo (0x65): запрос краткой информации о клане.
	r.registerStub(StateInGame, 0x65, "RequestPledgeInfo")
	// RequestPledgeExtendedInfo (0x66): запрос расширенной информации о клане.
	r.registerStub(StateInGame, 0x66, "RequestPledgeExtendedInfo")
	// RequestPledgeCrest (0x67): запрос герба клана.
	r.registerStub(StateInGame, 0x67, "RequestPledgeCrest")
	// RequestPledgePower (0xcc): управление полномочиями клана.
	r.registerStub(StateInGame, 0xcc, "RequestPledgePower")
	// RequestExPledgeCrestLarge (0xD0:0x10): запрос большого герба клана.
	r.registerMultiStub(StateInGame, 0x10, "RequestExPledgeCrestLarge")
	// RequestExSetPledgeCrestLarge (0xD0:0x11): установить большой герб клана.
	r.registerMultiStub(StateInGame, 0x11, "RequestExSetPledgeCrestLarge")
	// RequestPledgeSetAcademyMaster (0xD0:0x12): назначить мастера академии.
	r.registerMultiStub(StateInGame, 0x12, "RequestPledgeSetAcademyMaster")
	// RequestPledgePowerGradeList (0xD0:0x13): запрос списка рангов клана.
	r.registerMultiStub(StateInGame, 0x13, "RequestPledgePowerGradeList")
	// RequestPledgeMemberPowerInfo (0xD0:0x14): запрос полномочий члена клана.
	r.registerMultiStub(StateInGame, 0x14, "RequestPledgeMemberPowerInfo")
	// RequestPledgeSetMemberPowerGrade (0xD0:0x15): установить ранг члена клана.
	r.registerMultiStub(StateInGame, 0x15, "RequestPledgeSetMemberPowerGrade")
	// RequestPledgeMemberInfo (0xD0:0x16): запрос информации о члене клана.
	r.registerMultiStub(StateInGame, 0x16, "RequestPledgeMemberInfo")
	// RequestPledgeWarList (0xD0:0x17): запрос списка клановых войн.
	r.registerMultiStub(StateInGame, 0x17, "RequestPledgeWarList")
	// RequestPledgeReorganizeMember (0xD0:0x2c): реорганизация состава клана.
	r.registerMultiStub(StateInGame, 0x2c, "RequestPledgeReorganizeMember")
	// RequestExChangeName (0xD0:0x3b): смена имени персонажа.
	r.registerMultiStub(StateInGame, 0x3b, "RequestExChangeName")
}
