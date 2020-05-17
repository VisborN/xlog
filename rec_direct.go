package xlog

import (
	"fmt"
	"io"
	"math/rand"
	"sync"
)

type rqRecorderSignal string

type ioDirectRecorder struct {
	chCtl     chan ControlSignal  // receives a control signals
	chMsg     chan LogMsg         // receives a log message
	chErr     chan error          // returns a write errors
	chDbg     chan<- debugMessage // used for debug output
	chSyncErr chan error          // TMP

	isListening bool_s // internal mutex
	refCounter  int
	writer      io.Writer

	sync.RWMutex
	prefix string
	format FormatFunc
	closer func(interface{})
}

// NewIoDirectRecorder allocates and returns a new io direct recorder.
func NewIoDirectRecorder(
	writer io.Writer, prefix ...string,
) *ioDirectRecorder {
	r := new(ioDirectRecorder)
	//r.id = xid.NewWithTime(time.Now())
	r.chCtl = make(chan ControlSignal, 32)
	r.chMsg = make(chan LogMsg, 64)
	r.chSyncErr = make(chan error, 1)
	r.format = IoDirectDefaultFormatter
	r.writer = writer
	r.refCounter = 0
	if len(prefix) > 0 {
		r.prefix = prefix[0]
	}
	return r
}

func (R *ioDirectRecorder) GetChannels() ChanBundle {
	return ChanBundle{R.chCtl, R.chMsg, R.chSyncErr}
}

// InitErrChan initialises and returns an error channel to report write errors.
// If you use this function you must be sure, that some goroutine is reading the
// channel. Otherwise, you must drop it by DropErrChan function.
func (R *ioDirectRecorder) InitErrChan() <-chan error {
	if R.chErr == nil {
		R.chErr = make(chan error, 256)
		return R.chErr
	}
	return nil
}

func (R *ioDirectRecorder) SetDbgChan(ch chan<- debugMessage) {
	R.chDbg = ch
}

// DropErrChan closes error channel, the recorder will no more transmit write errors.
func (R *ioDirectRecorder) DropErrChan() {
	if R.chErr != nil {
		close(R.chErr)
		R.chErr = nil
	}
}

func (R *ioDirectRecorder) DropDbgChan() {
	if R.chDbg != nil {
		close(R.chDbg)
		R.chDbg = nil
	}
}

// FormatFunc sets custom formatter function for this recorder.
func (R *ioDirectRecorder) FormatFunc(f FormatFunc) *ioDirectRecorder {
	R.Lock()
	R.format = f
	R.Unlock()
	return R
}

// OnClose sets function which will be executed on close() function call.
func (R *ioDirectRecorder) OnClose(f func(interface{})) *ioDirectRecorder {
	R.Lock()
	R.closer = f
	R.Unlock()
	return R
}

func (R *ioDirectRecorder) ChangePrefixOnFly(prefix string) {
	R.Lock()
	defer R.Unlock()
	R.prefix = prefix
}

// -----------------------------------------------------------------------------

var dbg_rand_buffer []int

func (R *ioDirectRecorder) Listen() {
	if R.isListening.Get() {
		return
	} else {
		R.isListening.Set(true)
	}

get_rand:
	id := rand.Intn(10)
	for _, v := range dbg_rand_buffer {
		if v == id {
			goto get_rand
		}
	}
	dbg_rand_buffer = append(dbg_rand_buffer, id)

	R._log("start recorder listener, id=%d", id)

	for {
		select {
		case msg := <-R.chCtl:
			switch msg {
			case SignalInit:
				R._log("r%d | RECV INIT SIGNAL", id)
				R.initialise()
				R.chSyncErr <- nil // error ain't possible
			case SignalClose:
				R._log("r%d | RECV CLOSE SIGNAL", id)
				R.RLock()
				R.close()
				R.RUnlock()
				//R._log("r%d | .refCounter=%d", id, R.refCounter)
			case SignalStop: // TODO
				R._log("r%d | RECV STOP SIGNAL", id)
				R.isListening.Set(false)
				return
			default:
				R._log("r%d | RECV UNKNOWN SIGNAL", id)
				//R.chErr <- ErrUnknownSignal
				// unknown signal, skip
			}
		case msg := <-R.chMsg:
			R._log("r%d | RECV MSG", id)
			err := R.write(msg)
			if err != nil {
				R._log("r%d | ERR: %s", id, err.Error())
				if R.chErr != nil {
					R.chErr <- fmt.Errorf("[r%d] %s", id, err.Error())
				}
			}
		}
	}
}

func (R *ioDirectRecorder) IsListening() bool {
	return R.isListening.Get() // rc safe
}

// ----------------------------------------

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

// ----------------------------------------

func (R *ioDirectRecorder) write(msg LogMsg) error {
	if R.refCounter == 0 {
		return ErrNotInitialised
	}
	msgData := msg.content
	R.RLock()
	if R.format != nil {
		msgData = R.format(&msg)
	}
	if R.prefix != "" {
		msgData = fmt.Sprintf("%s %s", R.prefix, msgData)
	}
	R.RUnlock()
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

// -----------------------------------------------------------------------------

// TODO: more flags
// TODO: file & line
func IoDirectDefaultFormatter(msg *LogMsg) string {
	// short date/time format
	h, m, s := msg.GetTime().Clock()
	yy, mm, dd := msg.GetTime().Date()
	return fmt.Sprintf("%4d/%02d/%02d %02d:%02d:%02d %s %s",
		yy, mm, dd, h, m, s, msg.GetFlags().String(), msg.GetContent())
}
