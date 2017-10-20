package unilog

import "time"

type logMsg struct {
	level int
	msg   string
	when  time.Time
}

type Output interface {
	Init(config string) error
	WriteMsg(msg logMsg) error
	Destroy()
	Flush()
}

type newOutputFunc func() Output

var adapters = make(map[string]newOutputFunc)

func Register(name string, log newOutputFunc) {
	if log == nil {
		panic("logs: Register provide is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("logs: Register called twice for provider " + name)
	}
	adapters[name] = log
}
