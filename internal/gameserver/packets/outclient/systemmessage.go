package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// SystemMessage IDs (from L2J SystemMessageId.java).
const (
	SysMsgEarnedS1Exp            = 45  // "You have earned $s1 experience."
	SysMsgEarnedS1ExpAndS2SP     = 95  // "You have earned $s1 experience and $s2 SP."
	SysMsgYouIncreasedYourLevel  = 96  // "Your level has increased!"
	SysMsgCannotLogoutInCombat   = 101  // CANT_LOGOUT_WHILE_FIGHTING "You cannot exit the game while in combat."
	SysMsgCannotRestartInCombat  = 102  // CANT_RESTART_WHILE_FIGHTING "You cannot restart while in combat."
	SysMsgTargetNotFound         = 145  // TARGET_IS_NOT_FOUND_IN_THE_GAME (TELL to offline player)
	SysMsgNotEnoughMp            = 24   // NOT_ENOUGH_MP "Not enough MP."
	SysMsgDontSpam               = 1078 // DONT_SPAM "Please refrain from constant individual purchases."

	// Soulshot / Spiritshot messages (L2J SystemMessageId).
	SysMsgUseS1                  = 936 // USE_S1_ "You are using $s1." (item-name param)
	SysMsgSoulshotsGradeMismatch = 337 // SOULSHOTS_GRADE_MISMATCH
	SysMsgNotEnoughSoulshots     = 338 // NOT_ENOUGH_SOULSHOTS
	SysMsgCannotUseSoulshots     = 339 // CANNOT_USE_SOULSHOTS
	SysMsgEnabledSoulshot        = 342 // ENABLED_SOULSHOT
	SysMsgSpiritshotsGradeMismatch = 530 // SPIRITSHOTS_GRADE_MISMATCH
	SysMsgNotEnoughSpiritshots     = 531 // NOT_ENOUGH_SPIRITSHOTS
	SysMsgCannotUseSpiritshots     = 532 // CANNOT_USE_SPIRITSHOTS
	SysMsgEnabledSpiritshot        = 533 // ENABLED_SPIRITSHOT

	SysMsgUseOfS1WillBeAuto    = 1433 // USE_OF_S1_WILL_BE_AUTO ($s1 auto-use enabled)
	SysMsgAutoUseOfS1Cancelled = 1434 // AUTO_USE_OF_S1_CANCELLED ($s1 auto-use disabled)

	// Combat damage messages (L2J HF). Player->mob goes to the attacker; mob->player
	// goes to the victim. Verified against L2PcInstance.sendDamageMessage / PcStatus.
	SysMsgC1DoneS3DamageToC2        = 2261 // C1_DONE_S3_DAMAGE_TO_C2 [PLAYER_NAME, NPC_NAME, INT]
	SysMsgC1ReceivedDamageS3FromC2  = 2262 // C1_RECEIVED_DAMAGE_OF_S3_FROM_C2 [TEXT, NPC_NAME, INT]
	SysMsgC1AttackWentAstray        = 2265 // C1_ATTACK_WENT_ASTRAY (miss) [PLAYER_NAME]
	SysMsgC1HadCriticalHit          = 2266 // C1_HAD_CRITICAL_HIT (crit) [PLAYER_NAME]
)

// SystemMessage parameter types.
const (
	smParamText       = 0  // string
	smParamInt        = 1  // int32
	smParamNpcName    = 2  // int32 (template ID + 1000000)
	smParamItemName   = 3  // int32
	smParamSkillName  = 4  // 2×int32
	smParamLong       = 6  // int64
	smParamPlayerName = 12 // string
)

// smParam holds a single SystemMessage parameter.
type smParam struct {
	ptype int32
	ival  int32
	lval  int64
	sval  string
	// skill: [2]int32
}

// SystemMessageBuilder builds a SystemMessage packet (opcode 0x62).
type SystemMessageBuilder struct {
	msgID  int32
	params []smParam
}

// NewSystemMessage starts building a SystemMessage with the given message ID.
func NewSystemMessage(msgID int32) *SystemMessageBuilder {
	return &SystemMessageBuilder{msgID: msgID}
}

// AddInt adds an int32 parameter (TYPE_INT_NUMBER).
func (b *SystemMessageBuilder) AddInt(v int32) *SystemMessageBuilder {
	b.params = append(b.params, smParam{ptype: smParamInt, ival: v})
	return b
}

// AddLong adds an int64 parameter (TYPE_LONG_NUMBER).
func (b *SystemMessageBuilder) AddLong(v int64) *SystemMessageBuilder {
	b.params = append(b.params, smParam{ptype: smParamLong, lval: v})
	return b
}

// AddItemName adds an item-name parameter (TYPE_ITEM_NAME). The client resolves
// the localized item name from the item id.
func (b *SystemMessageBuilder) AddItemName(itemID int32) *SystemMessageBuilder {
	b.params = append(b.params, smParam{ptype: smParamItemName, ival: itemID})
	return b
}

// AddNpcName adds an NPC-name parameter (TYPE_NPC_NAME). L2J encodes it as the
// template id offset by 1000000; the client maps that to the localized NPC name.
func (b *SystemMessageBuilder) AddNpcName(templateID int32) *SystemMessageBuilder {
	b.params = append(b.params, smParam{ptype: smParamNpcName, ival: 1000000 + templateID})
	return b
}

// AddPlayerName adds a player-name parameter (TYPE_PLAYER_NAME, sent as text).
func (b *SystemMessageBuilder) AddPlayerName(name string) *SystemMessageBuilder {
	b.params = append(b.params, smParam{ptype: smParamPlayerName, sval: name})
	return b
}

// AddString adds a text parameter (TYPE_TEXT).
func (b *SystemMessageBuilder) AddString(v string) *SystemMessageBuilder {
	b.params = append(b.params, smParam{ptype: smParamText, sval: v})
	return b
}

// Build serializes the SystemMessage packet.
func (b *SystemMessageBuilder) Build() []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x62)
	w.WriteD(b.msgID)
	w.WriteD(int32(len(b.params)))
	for _, p := range b.params {
		w.WriteD(p.ptype)
		switch p.ptype {
		case smParamText, smParamPlayerName:
			w.WriteS(p.sval)
		case smParamLong:
			w.WriteQ(p.lval)
		case smParamInt, smParamNpcName, smParamItemName:
			w.WriteD(p.ival)
		}
	}
	return w.Bytes()
}

// BuildSystemMessageNoParams builds a simple SystemMessage with no parameters.
func BuildSystemMessageNoParams(msgID int32) []byte {
	return NewSystemMessage(msgID).Build()
}
