package client

func init() { addStubRegistrator(registerFriendsStubs) }

// registerFriendsStubs регистрирует стаб-обработчики списка друзей (High Five).
func registerFriendsStubs(r *Registry) {
	// RequestFriendInvite (0x77): пригласить игрока в друзья.
	r.registerStub(StateInGame, 0x77, "RequestFriendInvite")
	// RequestAnswerFriendInvite (0x78): ответ на запрос дружбы.
	r.registerStub(StateInGame, 0x78, "RequestAnswerFriendInvite")
	// RequestFriendList (0x79): запросить список друзей.
	r.registerStub(StateInGame, 0x79, "RequestFriendList")
	// RequestFriendDel (0x7a): удалить игрока из друзей.
	r.registerStub(StateInGame, 0x7a, "RequestFriendDel")
}
