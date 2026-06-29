package client

func init() { addStubRegistrator(registerCastleFortressStubs) }

// registerCastleFortressStubs регистрирует стаб-обработчики пакетов замков и
// крепостей (High Five).
func registerCastleFortressStubs(r *Registry) {
	// RequestAllCastleInfo (0xD0:0x3c): запрос информации о всех замках.
	r.registerMultiStub(StateInGame, 0x3c, "RequestAllCastleInfo")
	// RequestAllFortressInfo (0xD0:0x3d): запрос информации о всех крепостях.
	r.registerMultiStub(StateInGame, 0x3d, "RequestAllFortressInfo")
	// RequestAllAgitInfo (0xD0:0x3e): запрос информации о всех clan halls.
	r.registerMultiStub(StateInGame, 0x3e, "RequestAllAgitInfo")
	// RequestGetBossRecord (0xD0:0x40): запрос записи о боссах.
	r.registerMultiStub(StateInGame, 0x40, "RequestGetBossRecord")
}
