package client

func init() { addStubRegistrator(registerCombatStubs) }

// registerCombatStubs регистрирует стаб-обработчики боевых пакетов (High Five).
// Пакеты пока только логируют факт получения; здесь же будет их логика.
func registerCombatStubs(r *Registry) {
	// Attack (0x01): авто-атака — клиент сообщает об атаке цели.
	r.registerStub(StateInGame, 0x01, "Attack")
	// AttackRequest (0x32): запрос атаки через интерфейс.
	r.registerStub(StateInGame, 0x32, "AttackRequest")
	// RequestMagicSkillUse (0x39): использование скилла.
	r.registerStub(StateInGame, 0x39, "RequestMagicSkillUse")
	// StartRotating (0x5b): начало поворота персонажа.
	r.registerStub(StateInGame, 0x5b, "StartRotating")
	// FinishRotating (0x5c): конец поворота персонажа.
	r.registerStub(StateInGame, 0x5c, "FinishRotating")
	// RequestRestartPoint (0x7d): выбор точки воскрешения после смерти.
	r.registerStub(StateInGame, 0x7d, "RequestRestartPoint")
	// RequestExMagicSkillUseGround (0xD0:0x44): применение скилла по точке на земле.
	r.registerMultiStub(StateInGame, 0x44, "RequestExMagicSkillUseGround")
	// RequestDispel (0xD0:0x4b): снять баф/дебаф.
	r.registerMultiStub(StateInGame, 0x4b, "RequestDispel")
}
