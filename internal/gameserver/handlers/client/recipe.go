package client

func init() { addStubRegistrator(registerRecipeStubs) }

// registerRecipeStubs регистрирует стаб-обработчики пакетов рецептов и крафт-магазинов (High Five).
func registerRecipeStubs(r *Registry) {
	// RequestRecipeBookOpen (0xb5): открыть книгу рецептов.
	r.registerStub(StateInGame, 0xb5, "RequestRecipeBookOpen")
	// RequestRecipeBookDestroy (0xb6): удалить рецепт из книги.
	r.registerStub(StateInGame, 0xb6, "RequestRecipeBookDestroy")
	// RequestRecipeItemMakeInfo (0xb7): информация о создании предмета по рецепту.
	r.registerStub(StateInGame, 0xb7, "RequestRecipeItemMakeInfo")
	// RequestRecipeItemMakeSelf (0xb8): создать предмет по рецепту самому.
	r.registerStub(StateInGame, 0xb8, "RequestRecipeItemMakeSelf")
	// RequestRecipeShopMessageSet (0xba): задать сообщение крафт-магазина.
	r.registerStub(StateInGame, 0xba, "RequestRecipeShopMessageSet")
	// RequestRecipeShopListSet (0xbb): задать список крафт-магазина.
	r.registerStub(StateInGame, 0xbb, "RequestRecipeShopListSet")
	// RequestRecipeShopManageQuit (0xbc): закрыть управление крафт-магазином.
	r.registerStub(StateInGame, 0xbc, "RequestRecipeShopManageQuit")
	// RequestRecipeShopMakeInfo (0xbe): информация о крафт-магазине другого игрока.
	r.registerStub(StateInGame, 0xbe, "RequestRecipeShopMakeInfo")
	// RequestRecipeShopMakeItem (0xbf): заказать изготовление предмета в крафт-магазине.
	r.registerStub(StateInGame, 0xbf, "RequestRecipeShopMakeItem")
	// RequestRecipeShopManagePrev (0xc0): предыдущая страница крафт-магазина.
	r.registerStub(StateInGame, 0xc0, "RequestRecipeShopManagePrev")
}
