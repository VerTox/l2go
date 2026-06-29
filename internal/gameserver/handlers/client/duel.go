package client

func init() { addStubRegistrator(registerDuelStubs) }

// registerDuelStubs регистрирует стаб-обработчики пакетов дуэлей (High Five).
func registerDuelStubs(r *Registry) {
	// RequestDuelStart (0xD0:0x1b): вызвать игрока на дуэль.
	r.registerMultiStub(StateInGame, 0x1b, "RequestDuelStart")
	// RequestDuelAnswerStart (0xD0:0x1c): принять или отклонить вызов на дуэль.
	r.registerMultiStub(StateInGame, 0x1c, "RequestDuelAnswerStart")
	// RequestDuelSurrender (0xD0:0x45): сдаться в текущей дуэли.
	r.registerMultiStub(StateInGame, 0x45, "RequestDuelSurrender")
}
