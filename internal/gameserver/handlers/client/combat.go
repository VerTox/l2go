package client

func init() { addStubRegistrator(registerCombatStubs) }

// registerCombatStubs регистрирует стаб-обработчики боевых пакетов (High Five).
// Пакеты пока только логируют факт получения; здесь же будет их логика.
func registerCombatStubs(r *Registry) {
	// Attack (0x01): Ctrl force-attack по цели — реальный обработчик (l2go-npi).
	r.register(StateInGame, 0x01, "Attack", (*Handler).handleAttack)
	// AttackRequest (0x32): байт-идентичный дубликат Attack (L2J), тот же cddddc —
	// force-attack. Реальный HF-клиент шлёт 0x01, часть клиентов — 0x32. (l2go-npi)
	r.register(StateInGame, 0x32, "AttackRequest", (*Handler).handleAttack)
	// RequestMagicSkillUse (0x39) — реальный обработчик в cast.go (l2go-lu8).
	// StartRotating (0x5b): начало поворота персонажа.
	r.registerStub(StateInGame, 0x5b, "StartRotating")
	// FinishRotating (0x5c): конец поворота персонажа.
	r.registerStub(StateInGame, 0x5c, "FinishRotating")
	// RequestExMagicSkillUseGround (0xD0:0x44): применение скилла по точке на земле.
	r.registerMultiStub(StateInGame, 0x44, "RequestExMagicSkillUseGround")
	// RequestDispel (0xD0:0x4b) — реальный обработчик в cast.go (отмена баффа).
}
