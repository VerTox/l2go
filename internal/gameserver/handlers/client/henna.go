package client

func init() { addStubRegistrator(registerHennaStubs) }

// registerHennaStubs регистрирует стаб-обработчики пакетов татуировок/хны (High Five).
func registerHennaStubs(r *Registry) {
	// RequestHennaEquip (0x6f): нанести татуировку. Для системы хны.
	r.registerStub(StateInGame, 0x6f, "RequestHennaEquip")
	// RequestHennaRemoveList (0x70): список наносимых татуировок. Для системы хны.
	r.registerStub(StateInGame, 0x70, "RequestHennaRemoveList")
	// RequestHennaItemRemoveInfo (0x71): инфо об удаляемой татуировке. Для системы хны.
	r.registerStub(StateInGame, 0x71, "RequestHennaItemRemoveInfo")
	// RequestHennaRemove (0x72): удалить татуировку. Для системы хны.
	r.registerStub(StateInGame, 0x72, "RequestHennaRemove")
	// RequestHennaItemList (0xc3): список доступных татуировок. Для системы хны.
	r.registerStub(StateInGame, 0xc3, "RequestHennaItemList")
	// RequestHennaItemInfo (0xc4): инфо о татуировке. Для системы хны.
	r.registerStub(StateInGame, 0xc4, "RequestHennaItemInfo")
}
