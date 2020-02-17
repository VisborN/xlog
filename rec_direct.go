package xlog

import (
	"fmt"
	"io"
	"sync"
)

type ioDirectRecorder struct {
	sync.Mutex
	refCounter int
	prefix     string
	format     FormatFunc
	writer     io.Writer
	closer     func(interface{})
}

// NewIoDirectRecorder allocates and returns a new io direct recorder.
func NewIoDirectRecorder(writer io.Writer, prefix ...string) *ioDirectRecorder {
	r := new(ioDirectRecorder)
	r.format = IoDirectDefaultFormatter
	r.writer = writer
	r.refCounter = 0
	if len(prefix) > 0 {
		r.prefix = prefix[0]
	}
	return r
}

func (R *ioDirectRecorder) initialise() error {
	//if R.refCounter < 0 { R.refCounter = 0 }
	R.Lock()
	R.refCounter++
	R.Unlock()
	return nil
}

func (R *ioDirectRecorder) close() {
	R.Lock()
	defer R.Unlock()
	if R.refCounter == 0 {
		return
	}
	if R.refCounter == 1 {
		if R.closer != nil {
			R.closer(nil)
		}
	}
	R.refCounter--
}

// FormatFunc sets custom formatter function for this recorder.
func (R *ioDirectRecorder) FormatFunc(f FormatFunc) *ioDirectRecorder {
	R.Lock()
	defer R.Unlock()
	R.format = f
	return R
}

// OnClose sets function which will be executed on close() function call.
func (R *ioDirectRecorder) OnClose(f func(interface{})) *ioDirectRecorder {
	R.Lock()
	defer R.Unlock()
	R.closer = f
	return R
}

func (R *ioDirectRecorder) write(msg LogMsg) error {
	R.Lock()
	defer R.Unlock()
	if R.refCounter == 0 {
		return ErrNotInitialised
	}
	msgData := msg.content
	if R.format != nil {
		msgData = R.format(&msg)
	}
	if R.prefix != "" {
		msgData = fmt.Sprintf("%s %s", R.prefix, msgData)
	}
	if msgData[len(msgData)-1] != '\n' {
		msgData += "\n"
	}
	if _, err := R.writer.Write([]byte(msgData)); err != nil {
		return fmt.Errorf("writer error: %s", err.Error())
	}
	return nil
}

// TODO: more flags
// TODO: file & line
func IoDirectDefaultFormatter(msg *LogMsg) string {
	// short date/time format
	h, m, s := msg.GetTime().Clock()
	yy, mm, dd := msg.GetTime().Date()
	return fmt.Sprintf("%4d/%02d/%02d %02d:%02d:%02d %s %s",
		yy, mm, dd, h, m, s, msg.GetFlags().String(), msg.GetContent())
}
