package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// CharCreateFail (opcode 0x10) — character creation rejected. The reason code
// tells the client which localized message to show. Values match L2J
// CharCreateFail (High Five). (l2go-k9a)
const (
	CharCreateFailReasonCreationFailed   int32 = 0x00 // generic failure
	CharCreateFailReasonTooManyChars     int32 = 0x01 // account character limit reached
	CharCreateFailReasonNameExists       int32 = 0x02 // name already taken
	CharCreateFailReason16EngChars       int32 = 0x03 // name length / must be 1-16 english chars
	CharCreateFailReasonIncorrectName    int32 = 0x04 // invalid/forbidden name
	CharCreateFailReasonCreateNotAllowed int32 = 0x05 // creation disabled
	CharCreateFailReasonChooseAnotherSvr int32 = 0x06 // pick another server
)

// NewCharCreateFail builds the CharCreateFail packet for the given reason.
func NewCharCreateFail(reason int32) []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x10)
	b.WriteD(reason)
	return b.Bytes()
}
