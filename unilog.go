package unilog

import (
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	ADAPTER_CONSOLE = "console"
	ADAPTER_FILE    = "file"
)

type newLoggerFunc func() Logger

var adapters = make(map[string]newLoggerFunc)

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

func (m *UniLogger) setLogger(adapterName string, configs ...string) error {
	config := append(configs, "{}")[0]
	for _, l := range m.outputs {
		if l.name == adapterName {
			return fmt.Errorf("logs: duplicate adaptername %q (you have set this logger before)", adapterName)
		}
	}
	newLogger, ok := adapters[adapterName]
	if !ok {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}
	lg := newLogger()
	err := lg.Init(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "logs.BeeLogger.SetLogger: "+err.Error())
		return err
	}
	m.outputs = append(m.outputs, &nameLogger{name: adapterName, Logger: lg})
	return nil
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

func (m *UniLogger) DelLogger(adapterName string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	outputs := []*nameLogger{}
	for _, lg := range m.outputs {
		if lg.name == adapterName {
			lg.Destroy()
		} else {
			outputs = append(outputs, lg)
		}
	}
	if len(outputs) == len(m.outputs) {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}
	m.outputs = outputs
	return nil
}
