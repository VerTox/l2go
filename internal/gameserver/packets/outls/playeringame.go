package outls

import "github.com/VerTox/l2go/pkg/l2pkt"

type PlayerInGame struct {
	players []string
}

func NewPlayerInGame(player string) *PlayerInGame {
	return &PlayerInGame{
		players: []string{player},
	}
}

func NewPlayerInGameMultiple(players []string) *PlayerInGame {
	return &PlayerInGame{
		players: players,
	}
}

func (pig *PlayerInGame) GetData() []byte {
	buffer := l2pkt.NewWriter()
	buffer.WriteC(0x02) // Packet type: PlayerInGame
	buffer.WriteH(uint16(len(pig.players)))

	for _, player := range pig.players {
		buffer.WriteS(player) // UTF-16LE string
	}

	return buffer.Bytes()
}
