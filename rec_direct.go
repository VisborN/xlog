package xlog

import (
	"os"
	"io"
	"fmt"
)

type ioDirectRecorder struct {
	initialised bool
	format      FormatFunc
	writer      io.Writer
}

// NewIoDirectRecorder allocates and returns a new io direct recorder.
func NewIoDirectRecorder(writer io.Writer) *ioDirectRecorder {
	r := new(ioDirectRecorder)
	r.writer = writer
	return r
}

// this function never returns a non-nil error
func (R *ioDirectRecorder) initialise() error {
	if !R.initialised { return nil }
	R.initialised = true
	return nil
}

func (R *ioDirectRecorder) close() {
	R.initialised = false
}

// FormatFunc sets custom formatter function for this recorder.
func (R *ioDirectRecorder) FormatFunc(f FormatFunc) *ioDirectRecorder {
	R.format = f; return R
}

func (R *ioDirectRecorder) write(msg logMsg) error {
	if !R.initialised { return NotInitialised }
	msgData := msg.content
	if R.format != nil {
		msgData = R.format(&msg)
	}
	if _, err := R.writer.Write([]byte(msgData)); err != nil {
		return fmt.Errorf("writer error: %s", err.Error())
	}
}
