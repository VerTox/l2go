package client

func init() { addStubRegistrator(registerBoardStubs) }

// registerBoardStubs регистрирует стаб-обработчики пакетов BBS-доски и мини-карты (High Five).
func registerBoardStubs(r *Registry) {
	// RequestBBSwrite (0x24): написать сообщение на BBS-доску (community board).
	r.registerStub(StateInGame, 0x24, "RequestBBSwrite")
	// RequestShowBoard (0x5e): открыть/показать BBS-доску.
	r.registerStub(StateInGame, 0x5e, "RequestShowBoard")
	// RequestShowMiniMap (0x6c): показать мини-карту региона.
	r.registerStub(StateInGame, 0x6c, "RequestShowMiniMap")
}
