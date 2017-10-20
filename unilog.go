package unilog

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const (
	LevelEmergency = iota
	LevelAlert
	LevelCritical
	LevelError
	LevelWarning
	LevelNotice
	LevelInfo
	LevelDebug
)

var levelPrefix = [LevelDebug + 1]string{"[M] ", "[A] ", "[C] ", "[E] ", "[W] ", "[N] ", "[I] ", "[D] "}

const (
	adapterConsole = "console"
	adapterFile    = "file"
)

const levelLoggerImpl = -1

type newOutFunc func() Out

var adapters = make(map[string]newOutFunc)

//UniLogger is the main log struct
type Logger struct {
	lock          sync.Mutex
	level         int
	init          bool
	callDepthFlag bool
	callDepth     int
	outputs       []*nameOut
}

type nameOut struct {
	Out
	name string
}

func (m *Logger) setOutput(adapterName string, configs ...string) error {
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
		fmt.Fprintln(os.Stderr, "unilog: setOutput "+err.Error())
		return err
	}
	m.outputs = append(m.outputs, &nameOut{name: adapterName, Out: lg})
	return nil
}

//SetLogger func is set a adapter config for logger
func (m *Logger) SetOutput(adapterName string, configs ...string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if !m.init {
		m.outputs = []*nameOut{}
		m.init = true
	}
	return m.setOutput(adapterName, configs...)
}

//DelLogger func is delete a adapter config
func (m *Logger) DelOutput(adapterName string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	outputs := []*nameOut{}
	for _, lg := range m.outputs {
		if lg.name == adapterName {
			lg.Destroy()
		} else {
			outputs = append(outputs, lg)
		}
	}
	if len(outputs) == len(m.outputs) {
		return fmt.Errorf("logs: unknown %s adapter (forgotten Register?)", adapterName)
	}
	m.outputs = outputs
	return nil
}

func (m *Logger) writeToOutputs(msg logMsg) {
	for _, l := range m.outputs {
		err := l.WriteMsg(msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to WriteMsg to adapter:%v,error:%v\n", l.name, err)
		}
	}
}

func (m *Logger) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	// writeMsg will always add a '\n' character
	if p[len(p)-1] == '\n' {
		p = p[0 : len(p)-1]
	}
	// set levelLoggerImpl to ensure all log message will be write out
	err = m.writeMsg(levelLoggerImpl, string(p))
	if err == nil {
		return len(p), err
	}
	return 0, err
}

func (m *Logger) writeMsg(logLevel int, msg string, v ...interface{}) error {
	if !m.init {
		m.lock.Lock()
		m.setOutput(adapterConsole)
		m.lock.Unlock()
	}
	if len(v) > 0 {
		msg = fmt.Sprintf(msg, v...)
	}
	when := time.Now()
	if m.callDepthFlag {
		_, file, line, ok := runtime.Caller(m.callDepth)
		if !ok {
			file = "???"
			line = 0
		}
		_, filename := path.Split(file)
		msg = "[" + filename + ":" + strconv.Itoa(line) + "] " + msg
	}
	//set level info in front of filename info
	if logLevel == levelLoggerImpl {
		// set to emergency to ensure all log will be print out correctly
		logLevel = LevelEmergency
	} else {
		msg = levelPrefix[logLevel] + msg
	}
	m.writeToOutputs(logMsg{when: when, msg: msg, level: logLevel})
	return nil
}

func (m *Logger) SetLevel(l int) {
	m.level = l
}

func (m *Logger) SetCallDepth(d int) {
	m.callDepth = d
}

func (m *Logger) GetCallDepth() int {
	return m.callDepth
}

func (m *Logger) EnableCallDepth(b bool) {
	m.callDepthFlag = b
}
