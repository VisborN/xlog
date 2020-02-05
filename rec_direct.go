package xlog

import (
	"io"
	"fmt"
)

type ioDirectRecorder struct {
	initialised bool
	refCounter  int
	format      FormatFunc
	writer      io.Writer
	closer      func(interface{})
}

// NewIoDirectRecorder allocates and returns a new io direct recorder.
func NewIoDirectRecorder(writer io.Writer) *ioDirectRecorder {
	r := new(ioDirectRecorder)
	r.writer = writer
	r.refCounter = 0
	return r
}

// this function never returns a non-nil error
func (R *ioDirectRecorder) initialise() error {
	R.initialised = true
	R.refCounter++
	return nil
}

func (R *ioDirectRecorder) close() {
	if !R.initialised { return }
	if R.refCounter == 1 {
		if R.closer != nil { R.closer(nil) }
		R.initialised = false
	}; R.refCounter--
}

// FormatFunc sets custom formatter function for this recorder.
func (R *ioDirectRecorder) FormatFunc(f FormatFunc) *ioDirectRecorder {
	R.format = f; return R
}

// OnClose sets function which will be executed on close() function call.
func (R *ioDirectRecorder) OnClose(f func(interface{})) *ioDirectRecorder {
	R.closer = f; return R
}

func (R *ioDirectRecorder) write(msg LogMsg) error {
	if !R.initialised { return NotInitialised }
	msgData := msg.content
	if R.format != nil {
		msgData = R.format(&msg)
	}
	if msgData[len(msgData)-1] != '\n' {
		msgData += "\n"
	}
	if _, err := R.writer.Write([]byte(msgData)); err != nil {
		return fmt.Errorf("writer error: %s", err.Error())
	}
	return nil
}
