package outgs

import "github.com/VerTox/l2go/internal/loginserver/packets"

type InitLS struct {
	publicKey []byte
}

func NewInitLS(publicKey []byte) *InitLS {
	return &InitLS{
		publicKey: publicKey,
	}
}

func (ils *InitLS) GetData() []byte {
	buffer := new(packets.Buffer)
	buffer.WriteByte(0x00)                         // Packet type: InitLS
	buffer.WriteUInt32(uint32(len(ils.publicKey))) // Public key length (4 bytes)
	buffer.Write(ils.publicKey)                    // RSA public key

	return buffer.Bytes()
}
