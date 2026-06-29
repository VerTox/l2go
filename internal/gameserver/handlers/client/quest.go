package client

func init() { addStubRegistrator(registerQuestStubs) }

// registerQuestStubs регистрирует стаб-обработчики пакетов квестов (High Five).
func registerQuestStubs(r *Registry) {
	// RequestQuestList (0x62): список квестов персонажа. Для системы квестов.
	r.registerStub(StateInGame, 0x62, "RequestQuestList")
	// RequestQuestAbort (0x63): отменить квест. Для системы квестов.
	r.registerStub(StateInGame, 0x63, "RequestQuestAbort")
}
