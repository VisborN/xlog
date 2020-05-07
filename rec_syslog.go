package xlog

import (
	"errors"
	"log/syslog"
)

var errWrongPriority = errors.New("wrong priority value")

type syslogRecorder struct {
	chCtl     chan ControlSignal  // receives a control signals
	chMsg     chan LogMsg         // receives a log message
	chErr     chan error          // returns a write errors
	chDbg     chan<- debugMessage // used for debug output
	chSyncErr chan error          // TMP

	listening  bool
	refCounter int
	prefix     string
	format     FormatFunc
	logger     *syslog.Writer

	// says which function to use for each severity
	sevBindings map[MsgFlagT]syslog.Priority
}

// NewSyslogRecorder allocates and returns a new syslog recorder.
func NewSyslogRecorder(prefix string) *syslogRecorder {
	r := new(syslogRecorder)
	r.chCtl = make(chan ControlSignal, 32)
	r.chMsg = make(chan LogMsg, 256)
	r.chSyncErr = make(chan error, 1)
	r.refCounter = 0
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

func (R *syslogRecorder) InitErrChan() <-chan error {
	if R.chErr == nil {
		R.chErr = make(chan error, 256)
		return R.chErr
	}
	return nil
}

func (R *syslogRecorder) SetDbgChan(ch chan<- debugMessage) {
	R.chDbg = ch
}

func (R *syslogRecorder) DropErrChan() {
	if R.chErr != nil {
		close(R.chErr)
		R.chErr = nil
	}
}

func (R *syslogRecorder) DropDbgChan() {
	if R.chDbg != nil {
		close(R.chDbg)
		R.chDbg = nil
	}
}

func (R *syslogRecorder) GetChannels() ChanBundle {
	return ChanBundle{R.chCtl, R.chMsg, R.chSyncErr}
}

func (R *syslogRecorder) Listen() {
	if R.listening {
		return
	}
	R.listening = true

	R._log("start recorder listener")

	for {
		select {
		case msg := <-R.chCtl:
			switch msg {
			case SignalInit:
				R._log("RECV INIT SIGNAL")
				e := R.initialise()
				R.chSyncErr <- e
			case SignalClose:
				R._log("RECV CLOSE SIGNAL")
				R.close()
			case SignalStop:
				R._log("RECV STOP SIGNAL")
				R.listening = false
				return
			default:
				R._log("RECV UNKNOWN SIGNAL")
				//R.chErr <- ErrUnknownSignal
			}
		case msg := <-R.chMsg:
			R._log("RECV MSG")
			err := R.write(msg)
			if err != nil {
				R._log("ERR: %s", err.Error())
				if R.chErr != nil {
					R.chErr <- err
				}
			}
		}
	}
}

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

// BindSeverityFlag rebinds severity flag to the new syslog priority code.
func (R *syslogRecorder) BindSeverityFlag(severity MsgFlagT, priority syslog.Priority) error {
	severity = severity &^ SeverityShadowMask
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
	R.format = f
	return R
}

// DropDebugger sets debug channel to nil if it has been passed earlier.
// It allows the recorder to continue normal work when debug listener stopped.
func (R *syslogRecorder) DropDebugger() {
	R.chDbg = nil
}

func (R *syslogRecorder) write(msg LogMsg) error {
	if R.refCounter == 0 {
		return ErrNotInitialised
	}
	msgData := msg.content
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
		msg := DbgMsg(format, args...)
		R.chDbg <- msg
	}
}
