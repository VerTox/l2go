package client

func init() { addStubRegistrator(registerChatStubs) }

// registerChatStubs регистрирует стаб-обработчики пакетов чата и социальных
// действий (High Five). Пакеты пока только логируют факт получения; здесь же
// будет их логика.
func registerChatStubs(r *Registry) {
	// Say2 (0x49): чат (все каналы).
	r.registerStub(StateInGame, 0x49, "Say2")
	// RequestSendFriendMsg (0x6b): личное сообщение другу.
	r.registerStub(StateInGame, 0x6b, "RequestSendFriendMsg")
	// AnswerCoupleAction (0xD0:0x7a): ответ на совместное действие (эмот).
	r.registerMultiStub(StateInGame, 0x7a, "AnswerCoupleAction")
	// RequestVoteNew (0xD0:0x7e): голосование.
	r.registerMultiStub(StateInGame, 0x7e, "RequestVoteNew")
}
