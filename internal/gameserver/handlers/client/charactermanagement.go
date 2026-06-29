package client

func init() { addStubRegistrator(registerCharacterManagementStubs) }

// registerCharacterManagementStubs регистрирует стаб-обработчики управления персонажами (High Five).
func registerCharacterManagementStubs(r *Registry) {
	// CharacterRestore (0x7b): восстановление удалённого персонажа. Состояние Authed.
	r.registerStub(StateAuthed, 0x7b, "CharacterRestore")
}
