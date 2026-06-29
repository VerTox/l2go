package client

func init() { addStubRegistrator(registerCommandChannelStubs) }

// registerCommandChannelStubs регистрирует стаб-обработчики командного канала MPCC (High Five).
func registerCommandChannelStubs(r *Registry) {
	// RequestExAskJoinMPCC (0xD0:0x06): запросить вступление в командный канал.
	r.registerMultiStub(StateInGame, 0x06, "RequestExAskJoinMPCC")
	// RequestExAcceptJoinMPCC (0xD0:0x07): принять группу в командный канал.
	r.registerMultiStub(StateInGame, 0x07, "RequestExAcceptJoinMPCC")
	// RequestExOustFromMPCC (0xD0:0x08): исключить из командного канала.
	r.registerMultiStub(StateInGame, 0x08, "RequestExOustFromMPCC")
	// RequestExMPCCShowPartyMembersInfo (0xD0:0x2d): список участников канала.
	r.registerMultiStub(StateInGame, 0x2d, "RequestExMPCCShowPartyMembersInfo")
}
