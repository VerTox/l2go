package client

func init() { addStubRegistrator(registerMacroStubs) }

// registerMacroStubs регистрирует стаб-обработчики пакетов макросов (High Five).
func registerMacroStubs(r *Registry) {
	// RequestMakeMacro (0xcd): создать макрос. Для системы макросов.
	r.registerStub(StateInGame, 0xcd, "RequestMakeMacro")
	// RequestDeleteMacro (0xce): удалить макрос. Для системы макросов.
	r.registerStub(StateInGame, 0xce, "RequestDeleteMacro")
}
