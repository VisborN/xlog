package xlog

import (
	"errors"
	"log/syslog"
	"sync"
	"time"

	"github.com/rs/xid"
)

// TODO
var errWrongPriority = errors.New("wrong priority value")

type syslogRecorder struct {
	chCtl chan controlSignal
	chMsg chan LogMsg
	chErr chan<- error
	chDbg chan<- debugMessage

	id          xid.ID
	isListening bool_s // internal mutex
	refCounter  int
	prefix      string // can't be changeable
	logger      *syslog.Writer

	sync.RWMutex
	format FormatFunc

	// says which function to use for each severity
	sevBindings map[MsgFlagT]syslog.Priority
}

// NewSyslogRecorder allocates and returns a new syslog recorder.
func NewSyslogRecorder(prefix string) *syslogRecorder {
	r := new(syslogRecorder)
	r.id = xid.NewWithTime(time.Now())
	r.chCtl = make(chan controlSignal, 32)
	r.chMsg = make(chan LogMsg, 64)
	r.prefix = prefix
	r.sevBindings = make(map[MsgFlagT]syslog.Priority)

	// default bindings
	r.sevBindings[Emerg] = syslog.LOG_EMERG
	r.sevBindings[Alert] = syslog.LOG_ALERT
	r.sevBindings[Critical] = syslog.LOG_CRIT
	r.sevBindings[Error] = syslog.LOG_ERR
	r.sevBindings[Warning] = syslog.LOG_WARNING
	r.sevBindings[Notice] = syslog.LOG_NOTICE
	r.sevBindings[Info] = syslog.LOG_INFO
	r.sevBindings[Debug] = syslog.LOG_DEBUG

	r.sevBindings[CustomB1] = syslog.LOG_INFO
	r.sevBindings[CustomB2] = syslog.LOG_INFO

	return r
}

// SpawnSyslogRecorder creates recorder and starts a listener.
func SpawnSyslogRecorder(prefix string) *syslogRecorder {
	r := NewSyslogRecorder(prefix)
	go r.Listen()
	return r
}

// Intrf returns recorder's interface channels.
func (R *syslogRecorder) Intrf() RecorderInterface {
	return RecorderInterface{R.chCtl, R.chMsg}
}

// getID reeturns recorder's xid.
func (R *syslogRecorder) getID() xid.ID {
	return R.id
}

// BindSeverityFlag rebinds severity flag to the new syslog priority code.
func (R *syslogRecorder) BindSeverityFlag(severity MsgFlagT, priority syslog.Priority) error {
	severity = severity &^ SeverityShadowMask

	R.Lock()
	defer R.Unlock()

	if _, exist := R.sevBindings[severity]; !exist {
		return ErrWrongFlagValue
	}

	if // syslog.Priority can contains facility codes
	priority != syslog.LOG_EMERG &&
		priority != syslog.LOG_ALERT &&
		priority != syslog.LOG_CRIT &&
		priority != syslog.LOG_ERR &&
		priority != syslog.LOG_WARNING &&
		priority != syslog.LOG_NOTICE &&
		priority != syslog.LOG_INFO &&
		priority != syslog.LOG_DEBUG {
		return errWrongPriority
	}

	R.sevBindings[severity] = priority
	return nil
}

// FormatFunc sets custom formatter function for this recorder.
func (R *syslogRecorder) FormatFunc(f FormatFunc) *syslogRecorder {
	R.Lock()
	defer R.Unlock()
	R.format = f
	return R
}

// -----------------------------------------------------------------------------

func (R *syslogRecorder) Listen() {
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
				respErrChan := sig.data.(chan error) // MAP PANIC
				R._log("  chan: %v", respErrChan)
				e := R.initialise()
				R._log("  send response..")
				respErrChan <- e
				R._log("  done")
			case SigClose:
				R._log("RECV CLOSE SIGNAL")
				R.close() // rc safe
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
			R._log("RECV MSG SIGNAL <--\n  msg: %v", msg)
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

func (R *syslogRecorder) IsListening() bool {
	return R.isListening.Get() // rc safe
}

// ----------------------------------------

func (R *syslogRecorder) initialise() error {
	//if R.refCounter < 0 { R.refCounter = 0 }
	if R.refCounter == 0 {
		var err error
		R.logger, err = syslog.New(syslog.LOG_INFO|syslog.LOG_USER, R.prefix)
		if err != nil {
			return err
		}
	}
	R.refCounter++
	return nil
}

func (R *syslogRecorder) close() {
	if R.refCounter == 0 {
		return
	}
	if R.refCounter == 1 {
		R.logger.Close()
	}
	R.refCounter--
}

// ----------------------------------------

func (R *syslogRecorder) write(msg LogMsg) error {
	if R.refCounter == 0 {
		return ErrNotInitialised
	}
	msgData := msg.content

	R.RLock()
	defer R.RUnlock()

	if R.format != nil {
		msgData = R.format(&msg)
	}
	sev := msg.flags &^ SeverityShadowMask
	if priority, exist := R.sevBindings[sev]; exist {
		switch priority { // WRITE
		case syslog.LOG_EMERG:
			R.logger.Emerg(msgData)
		case syslog.LOG_ALERT:
			R.logger.Alert(msgData)
		case syslog.LOG_CRIT:
			R.logger.Crit(msgData)
		case syslog.LOG_ERR:
			R.logger.Err(msgData)
		case syslog.LOG_WARNING:
			R.logger.Warning(msgData)
		case syslog.LOG_NOTICE:
			R.logger.Notice(msgData)
		case syslog.LOG_INFO:
			R.logger.Info(msgData)
		case syslog.LOG_DEBUG:
			R.logger.Debug(msgData)
		default:
			return internalError(ieUnreachable, "unexpected priority value")
		}
	} else {
		return ErrWrongFlagValue
	}

	return nil
}

func (R *syslogRecorder) _log(format string, args ...interface{}) {
	if R.chDbg != nil {
		msg := DbgMsg(R.id, format, args...)
		msg.rtype = "syslogRecorder"
		R.chDbg <- msg
	}
}
