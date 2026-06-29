package client

func init() { addStubRegistrator(registerShortcutsStubs) }

// registerShortcutsStubs регистрирует стаб-обработчики пакетов шорткатов (High Five).
func registerShortcutsStubs(r *Registry) {
	// RequestShortCutReg (0x3d): зарегистрировать шорткат. Для системы шорткатов.
	r.registerStub(StateInGame, 0x3d, "RequestShortCutReg")
	// RequestShortCutDel (0x3f): удалить шорткат. Для системы шорткатов.
	r.registerStub(StateInGame, 0x3f, "RequestShortCutDel")
}
