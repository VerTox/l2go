package client

func init() { addStubRegistrator(registerNpcInteractionStubs) }

// registerNpcInteractionStubs регистрирует стаб-обработчики пакетов
// взаимодействия с NPC (High Five). Пакеты пока только логируют факт получения;
// здесь же будет их логика.
func registerNpcInteractionStubs(r *Registry) {
	// RequestLinkHtml (0x22): нажатие HTML-ссылки в NPC-диалоге.
	r.registerStub(StateInGame, 0x22, "RequestLinkHtml")
	// RequestBypassToServer (0x23) — реальный обработчик в skills_learn.go (l2go-hv9).
	// DlgAnswer (0xc6): ответ на системный диалог подтверждения.
	r.registerStub(StateInGame, 0xc6, "DlgAnswer")
	// BypassUserCmd (0xb3): пользовательская bypass-команда.
	r.registerStub(StateInGame, 0xb3, "BypassUserCmd")
}
