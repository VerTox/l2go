package client

func init() { addStubRegistrator(registerWarehouseStubs) }

// registerWarehouseStubs регистрирует стаб-обработчики пакетов склада (High Five).
func registerWarehouseStubs(r *Registry) {
	// SendWareHouseDepositList (0x3b): положить предметы на склад.
	r.registerStub(StateInGame, 0x3b, "SendWareHouseDepositList")
	// SendWareHouseWithDrawList (0x3c): взять предметы со склада.
	r.registerStub(StateInGame, 0x3c, "SendWareHouseWithDrawList")
}
