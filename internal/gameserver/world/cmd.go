package world

type Cmd interface{}

type CmdAttach struct {
	ConnID    int64
	AccountID int64
} // сессия подключена
type CmdEnterWorld struct {
	ConnID int64
	CharID int64
} // выбрать чара и зайти
type CmdMove struct {
	CharID           int64
	X, Y, Z, Heading int32
} // движение
type CmdSay struct {
	CharID  int64
	Text    string
	Channel int
}

type CmdAction struct{ CharID, TargetID int64 }
