package unilog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type fileOutput struct {
	sync.RWMutex         // write log order by order and  atomic incr maxLinesCurLines and maxSizeCurSize
	fileWriter           *os.File
	Filename             string `json:"filename"`
	MaxLines             int    `json:"maxlines"`
	maxLinesCurLines     int
	MaxSize              int `json:"maxsize"`
	maxSizeCurSize       int
	Daily                bool  `json:"daily"`
	MaxDays              int64 `json:"maxdays"`
	dailyOpenDate        int
	dailyOpenTime        time.Time
	Rotate               bool   `json:"rotate"`
	Level                int    `json:"level"`
	Perm                 string `json:"perm"`
	RotatePerm           string `json:"rotateperm"`
	fileNameOnly, suffix string // like "project.log", project is fileNameOnly and .log is suffix
}

// newFileWriter create a FileLogWriter returning as LoggerInterface.
func newFileOutput() Output {
	m := &fileOutput{
		Daily:      true,
		MaxDays:    7,
		Rotate:     true,
		RotatePerm: "0440",
		Level:      LevelDebug,
		Perm:       "0660",
	}
	return m
}

// Init file logger with json config.
// jsonConfig like:
//	{
//	"filename":"logs/beego.log",
//	"maxLines":10000,
//	"maxsize":1024,
//	"daily":true,
//	"maxDays":15,
//	"rotate":true,
//  	"perm":"0600"
//	}
func (m *fileOutput) Init(jsonConfig string) error {
	err := json.Unmarshal([]byte(jsonConfig), m)
	if err != nil {
		return err
	}
	if len(m.Filename) == 0 {
		return errors.New("config must have filename")
	}
	m.suffix = filepath.Ext(m.Filename)
	m.fileNameOnly = strings.TrimSuffix(m.Filename, m.suffix)
	if m.suffix == "" {
		m.suffix = ".log"
	}
	err = m.startOutput()
	return err
}

func (m *fileOutput) startOutput() error {
	file, err := m.createLogFile()
	if err != nil {
		return err
	}
	if m.fileWriter != nil {
		m.fileWriter.Close()
	}
	m.fileWriter = file
	return m.initFd()
}

func (m *fileOutput) initFd() error {
	fd := m.fileWriter
	fInfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("get stat err: %s", err)
	}
	m.maxSizeCurSize = int(fInfo.Size())
	m.dailyOpenTime = time.Now()
	m.dailyOpenDate = m.dailyOpenTime.Day()
	m.maxLinesCurLines = 0
	if m.Daily {
		go m.dailyRotate(m.dailyOpenTime)
	}
	if fInfo.Size() > 0 {
		count, err := m.lines()
		if err != nil {
			return err
		}
		m.maxLinesCurLines = count
	}
	return nil
}

func (m *fileOutput) lines() (int, error) {
	fd, err := os.Open(m.Filename)
	if err != nil {
		return 0, err
	}
	defer fd.Close()
	buf := make([]byte, 32768) // 32k
	count := 0
	lineSep := []byte{'\n'}
	for {
		c, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			return count, err
		}
		count += bytes.Count(buf[:c], lineSep)
		if err == io.EOF {
			break
		}
	}
	return count, nil
}

func (m *fileOutput) createLogFile() (*os.File, error) {
	perm, err := strconv.ParseInt(m.Perm, 8, 64)
	if err != nil {
		return nil, err
	}
	fd, err := os.OpenFile(m.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.FileMode(perm))
	if err == nil {
		os.Chmod(m.Filename, os.FileMode(perm))
	}
	return fd, err
}

func (m *fileOutput) needRotate(size int, day int) bool {
	return (m.MaxLines > 0 && m.maxLinesCurLines >= m.MaxLines) ||
		(m.MaxSize > 0 && m.maxSizeCurSize >= m.MaxSize) ||
		(m.Daily && day != m.dailyOpenDate)
}

func (m *fileOutput) dailyRotate(openTime time.Time) {
	year, month, day := openTime.Add(24 * time.Hour).Date()
	nextDay := time.Date(year, month, day, 0, 0, 0, 0, openTime.Location())
	tm := time.NewTimer(time.Duration(nextDay.UnixNano() - openTime.UnixNano() + 100))
	<-tm.C
	m.Lock()
	if m.needRotate(0, time.Now().Day()) {
		if err := m.doRotate(time.Now()); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", m.Filename, err)
		}
	}
	m.Unlock()
}

func (m *fileOutput) doRotate(logTime time.Time) error {
	num := 1
	fName := ""
	_, err := os.Lstat(m.Filename)
	if err != nil {
		goto RESTART_LOGGER
	}
	if m.MaxLines > 0 || m.MaxSize > 0 {
		for ; err == nil && num <= 999; num++ {
			fName = m.fileNameOnly + fmt.Sprintf(".%s.%03d%s", logTime.Format("2006-01-02"), num, m.suffix)
			_, err = os.Lstat(fName)
		}
	} else {
		fName = fmt.Sprintf("%s.%s%s", m.fileNameOnly, m.dailyOpenTime.Format("2006-01-02"), m.suffix)
		_, err = os.Lstat(fName)
		for ; err == nil && num <= 999; num++ {
			fName = m.fileNameOnly + fmt.Sprintf(".%s.%03d%s", m.dailyOpenTime.Format("2006-01-02"), num, m.suffix)
			_, err = os.Lstat(fName)
		}
	}
	if err == nil {
		return fmt.Errorf("Rotate: Cannot find free log number to rename %s", m.Filename)
	}
	m.fileWriter.Close()
	err = os.Rename(m.Filename, fName)
	if err != nil {
		goto RESTART_LOGGER
	}
	err = os.Chmod(fName, os.FileMode(0440))
	// re-start logger
RESTART_LOGGER:
	startLoggerErr := m.startOutput()
	go m.deleteOldLog()
	if startLoggerErr != nil {
		return fmt.Errorf("Rotate StartLogger: %s", startLoggerErr)
	}
	if err != nil {
		return fmt.Errorf("Rotate: %s", err)
	}
	return nil
}

func (m *fileOutput) deleteOldLog() {
	dir := filepath.Dir(m.Filename)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Unable to delete old log '%s', error: %v\n", path, r)
			}
		}()
		if info == nil {
			return
		}
		if !info.IsDir() && info.ModTime().Add(24*time.Hour*time.Duration(m.MaxDays)).Before(time.Now()) {
			if strings.HasPrefix(filepath.Base(path), filepath.Base(m.fileNameOnly)) &&
				strings.HasSuffix(filepath.Base(path), m.suffix) {
				os.Remove(path)
			}
		}
		return
	})
}

// WriteMsg write logger message into file.
func (m *fileOutput) WriteMsg(msg logMsg) error {
	if msg.level > m.Level {
		return nil
	}
	h := msg.when.Format("2006/01/02 15:04:05")
	d := msg.when.Day()
	body := h + msg.msg + "\n"
	if m.Rotate {
		m.RLock()
		if m.needRotate(len(body), d) {
			m.RUnlock()
			m.Lock()
			if m.needRotate(len(body), d) {
				if err := m.doRotate(msg.when); err != nil {
					fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", m.Filename, err)
				}
			}
			m.Unlock()
		} else {
			m.RUnlock()
		}
	}
	m.Lock()
	_, err := m.fileWriter.Write([]byte(body))
	if err == nil {
		m.maxLinesCurLines++
		m.maxSizeCurSize += len(body)
	}
	m.Unlock()
	return err
}

func (m *fileOutput) Destroy() {
	m.fileWriter.Close()
}

func (m *fileOutput) Flush() {
	m.fileWriter.Sync()
}

func init() {
	Register(adapterFile, newFileOutput)
}
