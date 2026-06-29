package client

func init() { addStubRegistrator(registerSecondPasswordStubs) }

// registerSecondPasswordStubs регистрирует стаб-обработчики второго пароля (High Five).
func registerSecondPasswordStubs(r *Registry) {
	// RequestEx2ndPasswordCheck (0xD0:0x93): проверка второго пароля. Состояние Authed.
	r.registerMultiStub(StateAuthed, 0x93, "RequestEx2ndPasswordCheck")
	// RequestEx2ndPasswordVerify (0xD0:0x94): верификация второго пароля. Состояние Authed.
	r.registerMultiStub(StateAuthed, 0x94, "RequestEx2ndPasswordVerify")
	// RequestEx2ndPasswordReq (0xD0:0x95): запрос второго пароля. Состояние Authed.
	r.registerMultiStub(StateAuthed, 0x95, "RequestEx2ndPasswordReq")
}
