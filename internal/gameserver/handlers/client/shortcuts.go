package client

func init() { addStubRegistrator(registerShortcutsStubs) }

// registerShortcutsStubs регистрирует обработчики пакетов шорткатов (High Five).
func registerShortcutsStubs(r *Registry) {
	// RequestShortCutReg (0x3d): сохранить шорткат на быстрой панели + echo ShortCutRegister.
	r.register(StateInGame, 0x3d, "RequestShortCutReg", (*Handler).handleRequestShortCutReg)
	// RequestShortCutDel (0x3f): удалить шорткат (ответ клиенту не нужен, L2J-паритет).
	r.register(StateInGame, 0x3f, "RequestShortCutDel", (*Handler).handleRequestShortCutDel)
}
