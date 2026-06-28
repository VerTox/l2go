package world

type Ev interface{}

type EvUserInfo struct {
	ConnID int64 /* поля ui */
}
type EvCharInfo struct {
	Receivers []int64 /* поля */
}
type EvCharMove struct {
	Receivers        []int64
	CharID           int64
	X, Y, Z, Heading int32
}
type EvChat struct {
	Receivers []int64
	From      int64
	Text      string
	Channel   int
}
