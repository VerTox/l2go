package client

func init() { addStubRegistrator(registerEventsMiscStubs) }

// registerEventsMiscStubs регистрирует стаб-обработчики событий и прочих пакетов (High Five).
func registerEventsMiscStubs(r *Registry) {
	// ObserverReturn (0xc1): выйти из режима наблюдателя. Состояние InGame.
	r.registerStub(StateInGame, 0xc1, "ObserverReturn")
	// RequestSSQStatus (0xc8): статус Seven Signs Quest. Состояние InGame.
	r.registerStub(StateInGame, 0xc8, "RequestSSQStatus")
	// SnoopQuit (0xb4): выйти из режима прослушивания. Состояние InGame.
	r.registerStub(StateInGame, 0xb4, "SnoopQuit")
	// RequestRecordInfo (0x6e): запросить запись (перезагрузка данных). Состояние InGame.
	r.registerStub(StateInGame, 0x6e, "RequestRecordInfo")
	// GameGuardReply (0xcb): ответ GameGuard анти-читу. Состояние InGame.
	r.registerStub(StateInGame, 0xcb, "GameGuardReply")
	// RequestWriteHeroWords (0xD0:0x05): записать слова героя. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x05, "RequestWriteHeroWords")
	// RequestExFishRanking (0xD0:0x18): рейтинг рыбалки. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x18, "RequestExFishRanking")
	// RequestPCCafeCouponUse (0xD0:0x19): использовать купон PC Cafe. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x19, "RequestPCCafeCouponUse")
	// RequestExRqItemLink (0xD0:0x1e): ссылка на предмет в чате. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x1e, "RequestExRqItemLink")
	// RequestCursedWeaponList (0xD0:0x2a): список проклятого оружия. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x2a, "RequestCursedWeaponList")
	// RequestCursedWeaponLocation (0xD0:0x2b): местоположение проклятого оружия. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x2b, "RequestCursedWeaponLocation")
	// RequestBidItemAuction (0xD0:0x39): ставка на аукционе предметов. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x39, "RequestBidItemAuction")
	// RequestInfoItemAuction (0xD0:0x3a): инфо о лоте аукциона. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x3a, "RequestInfoItemAuction")
	// RequestWithDrawPremiumItem (0xD0:0x52): получить премиум-предмет. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x52, "RequestWithDrawPremiumItem")
	// RequestExCleftEnter (0xD0:0x59): войти в Cleft (инстанс). Состояние InGame.
	r.registerMultiStub(StateInGame, 0x59, "RequestExCleftEnter")
	// RequestExCubeGameChangeTeam (0xD0:0x5a): сменить команду в Cube Game. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x5a, "RequestExCubeGameChangeTeam")
	// EndScenePlayer (0xD0:0x5b): завершить сцену игрока. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x5b, "EndScenePlayer")
	// RequestExCubeGameReadyAnswer (0xD0:0x5c): готовность в Cube Game. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x5c, "RequestExCubeGameReadyAnswer")
	// BrEventRankerList (0xD0:0x7b): рейтинг BR-события. Состояние InGame.
	r.registerMultiStub(StateInGame, 0x7b, "BrEventRankerList")
}
