package client

func init() { addStubRegistrator(registerTutorialStubs) }

// registerTutorialStubs регистрирует стаб-обработчики пакетов туториала
// (High Five). Пакеты пока только логируют факт получения; здесь же будет их
// логика.
func registerTutorialStubs(r *Registry) {
	// RequestTutorialLinkHtml (0x85): ссылка в туториале.
	r.registerStub(StateInGame, 0x85, "RequestTutorialLinkHtml")
	// RequestTutorialPassCmdToServer (0x86): команда туториала серверу.
	r.registerStub(StateInGame, 0x86, "RequestTutorialPassCmdToServer")
	// RequestTutorialQuestionMark (0x87): вопросительный знак туториала.
	r.registerStub(StateInGame, 0x87, "RequestTutorialQuestionMark")
	// RequestTutorialClientEvent (0x88): клиентское событие туториала.
	r.registerStub(StateInGame, 0x88, "RequestTutorialClientEvent")
}
