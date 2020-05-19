package xlog

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rs/xid"
)

type rqRecorderSignal string

type ioDirectRecorder struct {
	chCtl chan controlSignal
	chMsg chan LogMsg
	chErr chan<- error        // optional
	chDbg chan<- debugMessage // optional

	id          xid.ID
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
	r.id = xid.NewWithTime(time.Now())
	r.chCtl = make(chan controlSignal, 32)
	r.chMsg = make(chan LogMsg, 64)
	r.format = IoDirectDefaultFormatter
	r.writer = writer
	if len(prefix) > 0 {
		r.prefix = prefix[0]
	}
	return r
}

// Intrf returns recorder's interface channels.
func (R *ioDirectRecorder) Intrf() RecorderInterface {
	return RecorderInterface{R.chCtl, R.chMsg}
}

// getID returns recorder's xid.
func (R *ioDirectRecorder) getID() xid.ID {
	return R.id
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

func (R *ioDirectRecorder) Listen() {
	if R.isListening.Get() {
		return
	} else {
		R.isListening.Set(true)
		R._log("start listener...")
	}

	for {
		select {
		case sig := <-R.chCtl: // recv control signal
			switch sig.stype {
			case SigInit:
				R._log("RECV INIT SIGNAL")
				respErrChan := sig.data.(chan error) // MAY PANIC
				R._log("  chan: %v", respErrChan)
				R.initialise()
				R._log("  send response..")
				respErrChan <- nil // error ain't possible
				R._log("  done")
			case SigClose:
				R._log("RECV CLOSE SIGNAL")
				R.RLock()
				R.close()
				R.RUnlock()
			case SigStop:
				R._log("RECV STOP SIGNAL")
				R.isListening.Set(false)
				R._log("stop listener...")
				return

			case SigSetErrChan:
				R._log("RECV SET_ERR_CHAN SIGNAL")
				R.chErr = sig.data.(chan<- error) // MAY PANIC
			case SigSetDbgChan:
				R._log("RECV SET_DBG_CHAN SIGNAL")
				R.chDbg = sig.data.(chan<- debugMessage) // MAY PANIC
			case SigDropErrChan:
				R._log("RECV DROP_ERR_CHAN SIGNAL")
				//close(R.chErr)
				R.chErr = nil
			case SigDropDbgChan:
				R._log("RECV DROP_DBG_CHAN SIGNAL")
				//close(R.chDbg)
				R.chDbg = nil

			default:
				R._log("ERROR: received unknown signal (%s)", sig.stype)
				panic("xlog: received unknown signal") // PANIC
			}

		case msg := <-R.chMsg: // write log message
			R._log("RECV MSG SIGNAL <--\n  msg=%v", msg)
			err := R.write(msg)
			if err != nil {
				R._log("write error: %s", err.Error())
				if R.chErr != nil {
					R.chErr <- err // MAY PANIC
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
		return fmt.Errorf("writer fail: %s", err.Error())
	}
	return nil
}

func (R *ioDirectRecorder) _log(format string, args ...interface{}) { // MAY PANIC
	if R.chDbg != nil {
		msg := DbgMsg(R.id, format, args...)
		msg.rtype = "ioDirectRecorder"
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
