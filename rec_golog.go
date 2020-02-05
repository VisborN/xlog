package xlog

import (
	"io"
	"log"
	"fmt"
)

type gologRecorder struct {
	initialised bool
	refCounter  int
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
	r.refCounter = 0
	return r
}

func (R *gologRecorder) initialise() error {
	if !R.initialised {
		R.Logger = log.New(R.writer, R.prefix, log.LstdFlags)
		R.initialised = true
	}
	R.refCounter++
	return nil
}

func (R *gologRecorder) close() {
	if !R.initialised { return }
	if R.refCounter == 1 {
		if R.closer != nil { R.closer(nil) }
		R.initialised = false
	}; R.refCounter--
}

// FormatFunc sets custom formatter function for this recorder.
func (R *gologRecorder) FormatFunc(f FormatFunc) *gologRecorder {
	R.format = f; return R
}

// OnClose sets function which will be executed on close() function call.
func (R *gologRecorder) OnClose(f func(interface{})) *gologRecorder {
	R.closer = f; return R
}

func (R *gologRecorder) write(msg LogMsg) error {
	if !R.initialised { return NotInitialised }
	msgData := msg.content
	if R.format != nil {
		msgData = R.format(&msg)
	}
	R.Logger.Print(msgData)
	return nil
}

func GologDefaultFormatter(msg *LogMsg) string {
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
	case Custom1:  sevPrefix = "CUSTOM-1"
	case Custom2:  sevPrefix = "CUSTOM-2"
	case Custom3:  sevPrefix = "CUSTOM-3"
	case Custom4:  sevPrefix = "CUSTOM-4"
	default: sevPrefix = "<unknown>"
	}
	return fmt.Sprintf("%s :: %s", sevPrefix, msg.content)
}
