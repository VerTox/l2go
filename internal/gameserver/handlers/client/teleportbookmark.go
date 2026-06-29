package client

func init() { addStubRegistrator(registerTeleportBookmarkStubs) }

// registerTeleportBookmarkStubs регистрирует стаб-обработчики телепорт-закладок (High Five).
func registerTeleportBookmarkStubs(r *Registry) {
	// TeleportBookMark (0xD0:0x51): семейство телепорт-закладок; третий байт после
	// 0xD0:0x51 различает SlotInfo/Save/Modify/Delete/Teleport. Для стаба регистрируем
	// одну запись, разбор 3-го уровня будет при реализации. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x51, "TeleportBookMark")
}
