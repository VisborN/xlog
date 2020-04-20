package xlog

import (
	"fmt"
	"io"
)

type recorderSignal string

type ioDirectRecorder struct {
	chCtl chan ControlSignal
	chMsg chan *LogMsg
	chErr chan error

	listening  bool
	refCounter int
	prefix     string
	format     FormatFunc
	writer     io.Writer
	closer     func(interface{})
}

func (R *ioDirectRecorder) initChannels() {
	R.chCtl = make(chan ControlSignal, 256)
	R.chMsg = make(chan *LogMsg, 256)
	R.chErr = make(chan error, 256)
}

// NewIoDirectRecorder allocates and returns a new io direct recorder.
func NewIoDirectRecorder(
	writer io.Writer, errChannel chan error, prefix ...string,
) *ioDirectRecorder {
	r := new(ioDirectRecorder)
	r.chCtl = make(chan ControlSignal, 256)
	r.chMsg = make(chan *LogMsg, 256)
	if errChannel != nil {
		r.chErr = errChannel
	} else {
		r.chErr = DefErrChan
	}
	r.format = IoDirectDefaultFormatter
	r.writer = writer
	r.refCounter = 0
	if len(prefix) > 0 {
		r.prefix = prefix[0]
	}
	return r
}

func (R *ioDirectRecorder) GetChannels() ChanBundle {
	return ChanBundle{R.chCtl, R.chMsg, R.chErr}
}

func (R *ioDirectRecorder) Listen() {
	if R.listening {
		return
	}
	R.listening = true
	for {
		select {
		case msg := <-R.chCtl:
			switch msg {
			case SignalInit:
				R.initialise()
				R.chErr <- nil // error ain't possible
			case SignalClose:
				R.close()
			case SignalStop: // TODO
				R.listening = false
				return
			default:
				R.chErr <- ErrUnknownSignal
				// unknown signal, skip
			}
		case msg := <-R.chMsg:
			err := R.write(*msg)
			if err != nil {
				R.chErr <- err
			}
		}
	}
}

func (R *ioDirectRecorder) IsListening() bool {
	return R.listening
}

func (R *ioDirectRecorder) initialise() {
	R.refCounter++
	return
}

func (R *ioDirectRecorder) close() {
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
	R.format = f
	return R
}

// OnClose sets function which will be executed on close() function call.
func (R *ioDirectRecorder) OnClose(f func(interface{})) *ioDirectRecorder {
	R.closer = f
	return R
}

func (R *ioDirectRecorder) write(msg LogMsg) error {
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
