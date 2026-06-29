package client

func init() { addStubRegistrator(registerShopStubs) }

// registerShopStubs регистрирует стаб-обработчики пакетов торговли с NPC-магазином (High Five).
func registerShopStubs(r *Registry) {
	// RequestSellItem (0x37): продать предмет торговцу.
	r.registerStub(StateInGame, 0x37, "RequestSellItem")
	// RequestBuyItem (0x40): купить предмет у торговца.
	r.registerStub(StateInGame, 0x40, "RequestBuyItem")
	// RequestBuySeed (0xc5): купить семена (система Manor).
	r.registerStub(StateInGame, 0xc5, "RequestBuySeed")
	// RequestPackageSendableItemList (0xa7): список предметов для пакетной отправки.
	r.registerStub(StateInGame, 0xa7, "RequestPackageSendableItemList")
	// RequestPackageSend (0xa8): отправить пакет предметов.
	r.registerStub(StateInGame, 0xa8, "RequestPackageSend")
	// RequestRefundItem (0xD0:0x75): возврат предмета (buy-back).
	r.registerMultiStub(StateInGame, 0x75, "RequestRefundItem")
	// RequestBuySellUIClose (0xD0:0x76): закрыть UI покупки/продажи.
	r.registerMultiStub(StateInGame, 0x76, "RequestBuySellUIClose")
}
