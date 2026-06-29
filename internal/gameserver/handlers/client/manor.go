package client

func init() { addStubRegistrator(registerManorStubs) }

// registerManorStubs регистрирует стаб-обработчики пакетов системы манора (High Five).
func registerManorStubs(r *Registry) {
	// RequestProcureCropList (0xD0:0x02): запросить список закупки урожая в маноре.
	r.registerMultiStub(StateInGame, 0x02, "RequestProcureCropList")
	// RequestSetSeed (0xD0:0x03): установить семена для посева в маноре.
	r.registerMultiStub(StateInGame, 0x03, "RequestSetSeed")
	// RequestSetCrop (0xD0:0x04): установить урожай для продажи в маноре.
	r.registerMultiStub(StateInGame, 0x04, "RequestSetCrop")
	// RequestSeedPhase (0xD0:0x63): запросить текущую фазу семян в маноре.
	r.registerMultiStub(StateInGame, 0x63, "RequestSeedPhase")
}
