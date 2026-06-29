package client

func init() { addStubRegistrator(registerAllyStubs) }

// registerAllyStubs регистрирует стаб-обработчики пакетов альянса (High Five).
func registerAllyStubs(r *Registry) {
	// RequestAllyInfo (0x2e): запрос информации об альянсе.
	r.registerStub(StateInGame, 0x2e, "RequestAllyInfo")
	// RequestJoinAlly (0x8c): пригласить клан в альянс.
	r.registerStub(StateInGame, 0x8c, "RequestJoinAlly")
	// RequestAnswerJoinAlly (0x8d): ответ на приглашение в альянс.
	r.registerStub(StateInGame, 0x8d, "RequestAnswerJoinAlly")
	// AllyLeave (0x8e): выйти из альянса.
	r.registerStub(StateInGame, 0x8e, "AllyLeave")
	// AllyDismiss (0x8f): исключить клан из альянса.
	r.registerStub(StateInGame, 0x8f, "AllyDismiss")
	// RequestDismissAlly (0x90): распустить альянс.
	r.registerStub(StateInGame, 0x90, "RequestDismissAlly")
	// RequestSetAllyCrest (0x91): установить герб альянса.
	r.registerStub(StateInGame, 0x91, "RequestSetAllyCrest")
	// RequestAllyCrest (0x92): запрос герба альянса.
	r.registerStub(StateInGame, 0x92, "RequestAllyCrest")
}
