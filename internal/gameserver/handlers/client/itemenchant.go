package client

func init() { addStubRegistrator(registerItemEnchantStubs) }

// registerItemEnchantStubs регистрирует стаб-обработчики пакетов заточки и атрибутов предметов (High Five).
func registerItemEnchantStubs(r *Registry) {
	// RequestEnchantItem (0x5f): заточка предмета свитком.
	r.registerStub(StateInGame, 0x5f, "RequestEnchantItem")
	// RequestExEnchantItemAttribute (0xD0:0x35): зачаровать атрибут (стихию) предмета.
	r.registerMultiStub(StateInGame, 0x35, "RequestExEnchantItemAttribute")
	// RequestExRemoveItemAttribute (0xD0:0x23): удалить атрибут (стихию) предмета.
	r.registerMultiStub(StateInGame, 0x23, "RequestExRemoveItemAttribute")
	// RequestConfirmTargetItem (0xD0:0x26): подтвердить целевой предмет (Refinery).
	r.registerMultiStub(StateInGame, 0x26, "RequestConfirmTargetItem")
	// RequestConfirmRefinerItem (0xD0:0x27): подтвердить предмет рафинирования.
	r.registerMultiStub(StateInGame, 0x27, "RequestConfirmRefinerItem")
	// RequestConfirmGemStone (0xD0:0x28): подтвердить гемстон для рафинирования.
	r.registerMultiStub(StateInGame, 0x28, "RequestConfirmGemStone")
	// RequestRefine (0xD0:0x41): рафинировать предмет (Life Stone).
	r.registerMultiStub(StateInGame, 0x41, "RequestRefine")
	// RequestConfirmCancelItem (0xD0:0x42): подтвердить отмену (Refinery).
	r.registerMultiStub(StateInGame, 0x42, "RequestConfirmCancelItem")
	// RequestRefineCancel (0xD0:0x43): отменить рафинирование.
	r.registerMultiStub(StateInGame, 0x43, "RequestRefineCancel")
	// RequestExTryToPutEnchantTargetItem (0xD0:0x4c): поместить целевой предмет в окно прокачки.
	r.registerMultiStub(StateInGame, 0x4c, "RequestExTryToPutEnchantTargetItem")
	// RequestExTryToPutEnchantSupportItem (0xD0:0x4d): поместить вспомогательный предмет в прокачку.
	r.registerMultiStub(StateInGame, 0x4d, "RequestExTryToPutEnchantSupportItem")
	// RequestExCancelEnchantItem (0xD0:0x4e): отменить процесс зачарования.
	r.registerMultiStub(StateInGame, 0x4e, "RequestExCancelEnchantItem")
}
