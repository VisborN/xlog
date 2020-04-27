package xlog

import (
	"fmt"
	"io"
	"math/rand"
)

type recorderSignal string

type ioDirectRecorder struct {
	// We used two-way channels here (for convenience), so
	// you should be careful at using it inside the recorder.

	chCtl chan ControlSignal  // receives a control signals
	chMsg chan *LogMsg        // receives a log message
	chErr chan error          // returns status feedback for some actions
	chDbg chan<- debugMessage // used for debuging output if specified

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
	writer io.Writer, errChannel chan error, dbgChannel chan<- debugMessage, prefix ...string,
) *ioDirectRecorder {
	r := new(ioDirectRecorder)
	r.chCtl = make(chan ControlSignal, 256)
	r.chMsg = make(chan *LogMsg, 256)
	r.chDbg = dbgChannel
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

var dbg_rand_buffer []int

func (R *ioDirectRecorder) Listen() {
	if R.listening {
		return
	}
	R.listening = true

get_rand:
	id := rand.Intn(10)
	for _, v := range dbg_rand_buffer {
		if v == id {
			goto get_rand
		}
	}
	dbg_rand_buffer = append(dbg_rand_buffer, id)

	R._log("start recorder listener, id=%d", id) // TEMPORARY

	for {
		select {
		case msg := <-R.chCtl:
			switch msg {
			case SignalInit:
				R._log("r%d | RECV INIT SIGNAL", id)
				R.initialise()
				R.chErr <- nil // error ain't possible
			case SignalClose:
				R._log("r%d | RECV CLOSE SIGNAL", id)
				R.close()
				//R._log("r%d | .refCounter=%d", id, R.refCounter)
			case SignalStop: // TODO
				R._log("r%d | RECV STOP SIGNAL", id)
				R.listening = false
				return
			default:
				R._log("r%d | RECV UNKNOWN SIGNAL", id)
				R.chErr <- ErrUnknownSignal
				// unknown signal, skip
			}
		case msg := <-R.chMsg:
			R._log("r%d | RECV MSG", id)
			err := R.write(*msg)
			if err != nil {
				R._log("r%d | ERR: %s", id, err.Error())
				R.chErr <- fmt.Errorf("[r%d] %s", id, err.Error())
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

// DropDebugger sets debug channel to nil if it has been passed earlier.
// It allows the recorder to continue normal work when debug listener stopped.
func (R *ioDirectRecorder) DropDebugger() {
	R.chDbg = nil
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

func (R *ioDirectRecorder) _log(format string, args ...interface{}) {
	if R.chDbg != nil {
		msg := DbgMsg(format, args...)
		R.chDbg <- msg
	}
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
