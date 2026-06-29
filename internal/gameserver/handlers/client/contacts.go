package client

func init() { addStubRegistrator(registerContactsStubs) }

// registerContactsStubs регистрирует стаб-обработчики списка контактов (High Five).
func registerContactsStubs(r *Registry) {
	// RequestExAddContactToContactList (0xD0:0x84): добавить игрока в контакты.
	r.registerMultiStub(StateInGame, 0x84, "RequestExAddContactToContactList")
	// RequestExDeleteContactFromContactList (0xD0:0x85): удалить из контактов.
	r.registerMultiStub(StateInGame, 0x85, "RequestExDeleteContactFromContactList")
	// RequestExShowContactList (0xD0:0x86): показать список контактов.
	r.registerMultiStub(StateInGame, 0x86, "RequestExShowContactList")
	// RequestExFriendListExtended (0xD0:0x87): расширенный список друзей.
	r.registerMultiStub(StateInGame, 0x87, "RequestExFriendListExtended")
}
