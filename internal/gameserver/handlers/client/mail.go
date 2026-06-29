package client

func init() { addStubRegistrator(registerMailStubs) }

// registerMailStubs регистрирует стаб-обработчики почтовой системы (High Five).
func registerMailStubs(r *Registry) {
	// RequestPostItemList (0xD0:0x65): список предметов, доступных для вложения в письмо.
	r.registerMultiStub(StateInGame, 0x65, "RequestPostItemList")
	// RequestSendPost (0xD0:0x66): отправить письмо.
	r.registerMultiStub(StateInGame, 0x66, "RequestSendPost")
	// RequestReceivedPostList (0xD0:0x67): список полученных писем.
	r.registerMultiStub(StateInGame, 0x67, "RequestReceivedPostList")
	// RequestDeleteReceivedPost (0xD0:0x68): удалить полученное письмо.
	r.registerMultiStub(StateInGame, 0x68, "RequestDeleteReceivedPost")
	// RequestReceivedPost (0xD0:0x69): прочитать полученное письмо.
	r.registerMultiStub(StateInGame, 0x69, "RequestReceivedPost")
	// RequestPostAttachment (0xD0:0x6a): получить вложение письма.
	r.registerMultiStub(StateInGame, 0x6a, "RequestPostAttachment")
	// RequestRejectPostAttachment (0xD0:0x6b): отклонить вложение письма.
	r.registerMultiStub(StateInGame, 0x6b, "RequestRejectPostAttachment")
	// RequestSentPostList (0xD0:0x6c): список отправленных писем.
	r.registerMultiStub(StateInGame, 0x6c, "RequestSentPostList")
	// RequestDeleteSentPost (0xD0:0x6d): удалить отправленное письмо.
	r.registerMultiStub(StateInGame, 0x6d, "RequestDeleteSentPost")
	// RequestSentPost (0xD0:0x6e): прочитать отправленное письмо.
	r.registerMultiStub(StateInGame, 0x6e, "RequestSentPost")
	// RequestCancelPostAttachment (0xD0:0x6f): отменить отправку вложения.
	r.registerMultiStub(StateInGame, 0x6f, "RequestCancelPostAttachment")
}
