package client

func init() { addStubRegistrator(registerVehicleStubs) }

// registerVehicleStubs регистрирует стаб-обработчики пакетов транспорта (High Five).
func registerVehicleStubs(r *Registry) {
	// RequestGetOnVehicle (0x53): сесть в транспорт. Для системы транспорта.
	r.registerStub(StateInGame, 0x53, "RequestGetOnVehicle")
	// RequestGetOffVehicle (0x54): выйти из транспорта. Для системы транспорта.
	r.registerStub(StateInGame, 0x54, "RequestGetOffVehicle")
	// RequestMoveToLocationInVehicle (0x75): движение внутри транспорта. Для системы транспорта.
	r.registerStub(StateInGame, 0x75, "RequestMoveToLocationInVehicle")
	// CannotMoveAnymoreInVehicle (0x76): остановка движения в транспорте. Для системы транспорта.
	r.registerStub(StateInGame, 0x76, "CannotMoveAnymoreInVehicle")
	// MoveToLocationInAirShip (0xD0:0x20): движение внутри дирижабля. Мультипакет 0xD0.
	r.registerMultiStub(StateInGame, 0x20, "MoveToLocationInAirShip")
	// ExGetOnAirShip (0xD0:0x36): сесть на дирижабль. Мультипакет 0xD0, в StateInGame
	// (тот же sub 0x36 в StateAuthed занят GotoLobby в фундаменте — здесь именно InGame).
	r.registerMultiStub(StateInGame, 0x36, "ExGetOnAirShip")
}
