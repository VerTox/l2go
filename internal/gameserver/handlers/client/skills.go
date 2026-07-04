package client

func init() { addStubRegistrator(registerSkillsStubs) }

// registerSkillsStubs регистрирует стаб-обработчики пакетов скиллов (High Five).
// Пакеты пока только логируют факт получения; здесь же будет их логика.
func registerSkillsStubs(r *Registry) {
	// RequestSkillList (0x50): запрос списка скиллов.
	r.registerStub(StateInGame, 0x50, "RequestSkillList")
	// RequestAcquireSkillInfo (0x73) / RequestAcquireSkill (0x7c) — реальные
	// обработчики в skills_learn.go (l2go-hv9).
	// RequestExEnchantSkillInfo (0xD0:0x0e): инфо о зачаровании скилла.
	r.registerMultiStub(StateInGame, 0x0e, "RequestExEnchantSkillInfo")
	// RequestExEnchantSkill (0xD0:0x0f): зачаровать скилл.
	r.registerMultiStub(StateInGame, 0x0f, "RequestExEnchantSkill")
	// RequestExEnchantSkillSafe (0xD0:0x32): безопасное зачарование скилла.
	r.registerMultiStub(StateInGame, 0x32, "RequestExEnchantSkillSafe")
	// RequestExEnchantSkillUntrain (0xD0:0x33): разучить зачарование.
	r.registerMultiStub(StateInGame, 0x33, "RequestExEnchantSkillUntrain")
	// RequestExEnchantSkillRouteChange (0xD0:0x34): смена маршрута зачарования.
	r.registerMultiStub(StateInGame, 0x34, "RequestExEnchantSkillRouteChange")
	// RequestExEnchantSkillInfoDetail (0xD0:0x46): детали зачарования скилла.
	r.registerMultiStub(StateInGame, 0x46, "RequestExEnchantSkillInfoDetail")
}
