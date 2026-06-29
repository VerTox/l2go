package client

func init() { addStubRegistrator(registerTradeStubs) }

// registerTradeStubs регистрирует стаб-обработчики пакетов обмена между игроками (High Five).
func registerTradeStubs(r *Registry) {
	// TradeRequest (0x1a): инициировать обмен с игроком.
	r.registerStub(StateInGame, 0x1a, "TradeRequest")
	// AddTradeItem (0x1b): добавить предмет в окно обмена.
	r.registerStub(StateInGame, 0x1b, "AddTradeItem")
	// TradeDone (0x1c): подтвердить/завершить обмен.
	r.registerStub(StateInGame, 0x1c, "TradeDone")
	// AnswerTradeRequest (0x55): принять/отклонить запрос обмена.
	r.registerStub(StateInGame, 0x55, "AnswerTradeRequest")
}
