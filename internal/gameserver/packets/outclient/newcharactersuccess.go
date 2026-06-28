package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// CharTemplate represents a character creation template
type CharTemplate struct {
	Race      int32 `json:"race"`
	ClassID   int32 `json:"class_id"`
	STR       int32 `json:"str"`
	DEX       int32 `json:"dex"`
	CON       int32 `json:"con"`
	INT       int32 `json:"int"`
	WIT       int32 `json:"wit"`
	MEN       int32 `json:"men"`
	StartingX int32 `json:"starting_x"`
	StartingY int32 `json:"starting_y"`
	StartingZ int32 `json:"starting_z"`
	MaxHP     int32 `json:"max_hp"`
	MaxMP     int32 `json:"max_mp"`
}

// NewCharacterSuccess – response to character template request (opcode 0x0d).
func NewCharacterSuccess() []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x0d)
	b.WriteD(0x00) // No templates
	return b.Bytes()
}

// NewCharacterSuccessWithTemplates – response with character templates
func NewCharacterSuccessWithTemplates(templates []CharTemplate) []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x0d)
	b.WriteD(int32(len(templates)))

	for _, template := range templates {
		b.WriteD(int32(template.Race))
		b.WriteD(int32(template.ClassID))
		b.WriteD(int32(template.STR))
		b.WriteD(int32(template.DEX))
		b.WriteD(int32(template.CON))
		b.WriteD(int32(template.INT))
		b.WriteD(int32(template.WIT))
		b.WriteD(int32(template.MEN))
		b.WriteD(int32(template.StartingX))
		b.WriteD(int32(template.StartingY))
		b.WriteD(int32(template.StartingZ))
		b.WriteD(int32(template.MaxHP))
		b.WriteD(int32(template.MaxMP))
	}

	return b.Bytes()
}
