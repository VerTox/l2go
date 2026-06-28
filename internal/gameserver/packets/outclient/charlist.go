package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// CharList – минимальная заглушка списка персонажей (opcode 0x1f).
// Для стартовой интеграции возвращаем пустой список.
func NewCharList() []byte {
	b := l2pkt.NewWriter()
	b.WriteC(0x1f)
	// Заглушка: 4 байта нулей как в legacy версии
	b.WriteB([]byte{0x00, 0x00, 0x00, 0x00})
	return b.Bytes()
}
