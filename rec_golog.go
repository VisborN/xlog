package xlog

import (
	"io"
	"log"
	"fmt"
)

type gologRecorder struct {
	initialised bool
	prefix      string
	format      FormatFunc
	writer      io.Writer
	Logger      *log.Logger
	closer      func(interface{})
}

// NewGologRecorder allocates and returns a new recorder 
// which uses standart go log package for writing.
func NewGologRecorder(writer io.Writer, prefix string) *gologRecorder {
	r := new(gologRecorder)
	r.format = GologDefaultFormatter
	r.prefix = prefix + " "
	r.writer = writer
	return r
}

func (R *gologRecorder) initialise() error {
	if R.initialised { return nil }
	R.Logger = log.New(R.writer, R.prefix, log.LstdFlags)
	R.initialised = true
	return nil
}

func (R *gologRecorder) close() {
	if R.closer != nil { R.closer(nil) }
	R.initialised = false
}

// FormatFunc sets custom formatter function for this recorder.
func (R *gologRecorder) FormatFunc(f FormatFunc) *gologRecorder {
	R.format = f; return R
}

// OnClose sets function which will be executed on close() function call.
func (R *gologRecorder) OnClose(f func(interface{})) *gologRecorder {
	R.closer = f; return R
}

func (R *gologRecorder) write(msg logMsg) error {
	if !R.initialised { return NotInitialised }
	msgData := msg.content
	if R.format != nil {
		msgData = R.format(&msg)
	}
	R.Logger.Print(msgData)
	return nil
}

func GologDefaultFormatter(msg *logMsg) string {
	var sevPrefix string
	switch msg.severity {
	case Critical: sevPrefix = "CRITICAL"
	case Error:    sevPrefix = "ERROR"
	case Warning:  sevPrefix = "WARNING"
	case Notice:   sevPrefix = "NOTICE"
	case Info:     sevPrefix = "INFO"
	case Debug1:   sevPrefix = "DEBUG-1"
	case Debug2:   sevPrefix = "DEBUG-2"
	case Debug3:   sevPrefix = "DEBUG-3"
	default: sevPrefix = "<unknown>"
	}
	return fmt.Sprintf("%s :: %s", sevPrefix, msg.content)
}
