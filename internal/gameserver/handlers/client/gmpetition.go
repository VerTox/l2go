package client

func init() { addStubRegistrator(registerGmPetitionStubs) }

// registerGmPetitionStubs регистрирует стаб-обработчики пакетов GM-команд и петиций (High Five).
func registerGmPetitionStubs(r *Registry) {
	// SendBypassBuildCmd (0x74): выполнить GM bypass-команду (//-команды билдера).
	r.registerStub(StateInGame, 0x74, "SendBypassBuildCmd")
	// RequestGMCommand (0x7e): выполнить команду GM над целевым игроком.
	r.registerStub(StateInGame, 0x7e, "RequestGMCommand")
	// RequestGmList (0x8b): запросить список GM, находящихся онлайн.
	r.registerStub(StateInGame, 0x8b, "RequestGmList")
	// RequestPetition (0x89): создать петицию (обращение) к GM.
	r.registerStub(StateInGame, 0x89, "RequestPetition")
	// RequestPetitionCancel (0x8a): отменить ранее созданную петицию.
	r.registerStub(StateInGame, 0x8a, "RequestPetitionCancel")
	// RequestPetitionFeedback (0xc9): отправить обратную связь по обработанной петиции.
	r.registerStub(StateInGame, 0xc9, "RequestPetitionFeedback")
}
