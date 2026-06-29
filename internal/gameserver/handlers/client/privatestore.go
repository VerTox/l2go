package client

func init() { addStubRegistrator(registerPrivateStoreStubs) }

// registerPrivateStoreStubs регистрирует стаб-обработчики пакетов личных магазинов (High Five).
func registerPrivateStoreStubs(r *Registry) {
	// RequestPrivateStoreManageSell (0x30): открыть управление магазином продажи.
	r.registerStub(StateInGame, 0x30, "RequestPrivateStoreManageSell")
	// SetPrivateStoreListSell (0x31): задать список товаров на продажу.
	r.registerStub(StateInGame, 0x31, "SetPrivateStoreListSell")
	// RequestPrivateStoreBuy (0x83): купить в личном магазине продавца.
	r.registerStub(StateInGame, 0x83, "RequestPrivateStoreBuy")
	// RequestPrivateStoreQuitSell (0x96): закрыть магазин продажи.
	r.registerStub(StateInGame, 0x96, "RequestPrivateStoreQuitSell")
	// SetPrivateStoreMsgSell (0x97): задать сообщение магазина продажи.
	r.registerStub(StateInGame, 0x97, "SetPrivateStoreMsgSell")
	// RequestPrivateStoreManageBuy (0x99): открыть управление магазином покупки.
	r.registerStub(StateInGame, 0x99, "RequestPrivateStoreManageBuy")
	// SetPrivateStoreListBuy (0x9a): задать список в магазине покупки.
	r.registerStub(StateInGame, 0x9a, "SetPrivateStoreListBuy")
	// RequestPrivateStoreQuitBuy (0x9c): закрыть магазин покупки.
	r.registerStub(StateInGame, 0x9c, "RequestPrivateStoreQuitBuy")
	// SetPrivateStoreMsgBuy (0x9d): задать сообщение магазина покупки.
	r.registerStub(StateInGame, 0x9d, "SetPrivateStoreMsgBuy")
	// RequestPrivateStoreSell (0x9f): продать в личный магазин покупки (покупатель).
	r.registerStub(StateInGame, 0x9f, "RequestPrivateStoreSell")
	// SetPrivateStoreWholeMsg (0xD0:0x4a): сообщение для всего магазина.
	r.registerMultiStub(StateInGame, 0x4a, "SetPrivateStoreWholeMsg")
}
