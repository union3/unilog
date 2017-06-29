package unilog

import (
	"sync"
	"time"
)

const (
	ADAPTER_CONSOLE = "console"
	ADAPTER_FILE    = "file"
)

type UniLogger struct {
	lock    sync.Mutex
	level   int
	init    bool
	outputs []*nameLogger
}

type Logger interface {
	Init(config string) error
	WriteMsg(when time.Time, msg string, level int) error
	Destroy()
	Flush()
}

type nameLogger struct {
	Logger
	name string
}

func (m *UniLogger) SetLogger(adapterName string, configs ...string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if !m.init {
		m.outputs = []*nameLogger{}
		m.init = true
	}
	return nil
}
