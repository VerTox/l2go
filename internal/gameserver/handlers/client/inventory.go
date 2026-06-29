package client

func init() { addStubRegistrator(registerInventoryStubs) }

// registerInventoryStubs регистрирует стаб-обработчики пакетов инвентаря (High Five).
func registerInventoryStubs(r *Registry) {
	// RequestDropItem (0x17): выбросить предмет из инвентаря на землю.
	r.registerStub(StateInGame, 0x17, "RequestDropItem")
	// RequestDestroyItem (0x60): уничтожить предмет из инвентаря.
	r.registerStub(StateInGame, 0x60, "RequestDestroyItem")
	// RequestCrystallizeItem (0x2f): кристаллизовать предмет (получить кристаллы).
	r.registerStub(StateInGame, 0x2f, "RequestCrystallizeItem")
	// RequestPreviewItem (0xc7): предпросмотр/примерка предмета на персонаже.
	r.registerStub(StateInGame, 0xc7, "RequestPreviewItem")
	// MultiSellChoose (0xb0): выбор позиции в окне мультиселла.
	r.registerStub(StateInGame, 0xb0, "MultiSellChoose")
}
