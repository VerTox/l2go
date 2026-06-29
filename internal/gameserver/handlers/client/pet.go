package client

func init() { addStubRegistrator(registerPetStubs) }

// registerPetStubs регистрирует стаб-обработчики пакетов питомцев (High Five).
func registerPetStubs(r *Registry) {
	// RequestChangePetName (0x93): переименовать питомца. Для системы петов.
	r.registerStub(StateInGame, 0x93, "RequestChangePetName")
	// RequestPetUseItem (0x94): использовать предмет за питомца. Для системы петов.
	r.registerStub(StateInGame, 0x94, "RequestPetUseItem")
	// RequestGiveItemToPet (0x95): передать предмет питомцу. Для системы петов.
	r.registerStub(StateInGame, 0x95, "RequestGiveItemToPet")
	// RequestPetGetItem (0x98): питомец подбирает предмет. Для системы петов.
	r.registerStub(StateInGame, 0x98, "RequestPetGetItem")
	// RequestGetItemFromPet (0x2c): забрать предмет у питомца. Для системы петов.
	r.registerStub(StateInGame, 0x2c, "RequestGetItemFromPet")
}
