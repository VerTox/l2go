package client

func init() { addStubRegistrator(registerOlympiadStubs) }

// registerOlympiadStubs регистрирует стаб-обработчики пакетов Олимпиады (High Five).
func registerOlympiadStubs(r *Registry) {
	// RequestOlympiadObserverEnd (0xD0:0x29): выйти из режима наблюдения за матчем Олимпиады.
	r.registerMultiStub(StateInGame, 0x29, "RequestOlympiadObserverEnd")
	// RequestOlympiadMatchList (0xD0:0x2e): запросить список текущих матчей Олимпиады.
	r.registerMultiStub(StateInGame, 0x2e, "RequestOlympiadMatchList")
	// RequestExOlympiadMatchListRefresh (0xD0:0x88): обновить список матчей Олимпиады.
	r.registerMultiStub(StateInGame, 0x88, "RequestExOlympiadMatchListRefresh")
}
