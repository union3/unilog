package unilog

import "time"

type logMsg struct {
	level int
	msg   string
	when  time.Time
}

type Out interface {
	Init(config string) error
	WriteMsg(msg logMsg) error
	Destroy()
	Flush()
}
