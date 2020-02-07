package xlog

import (
	"sync"
	"errors"
	"log/syslog"
)

var errWrongPriority = errors.New("wrong priority value")

type syslogRecorder struct {
	sync.Mutex
	refCounter  int
	prefix      string
	format      FormatFunc
	logger      *syslog.Writer

	// says which function to use for each severity
	sevBindings map[MsgFlagT]syslog.Priority
}

// NewSyslogRecorder allocates and returns a new syslog recorder.
func NewSyslogRecorder(prefix string) *syslogRecorder {
	r := new(syslogRecorder)
	r.refCounter = 0
	r.prefix = prefix
	r.sevBindings = make(map[MsgFlagT]syslog.Priority)

	// default bindings
	r.sevBindings[Emerg]    = syslog.LOG_EMERG
	r.sevBindings[Alert]    = syslog.LOG_ALERT
	r.sevBindings[Critical] = syslog.LOG_CRIT
	r.sevBindings[Error]    = syslog.LOG_ERR
	r.sevBindings[Warning]  = syslog.LOG_WARNING
	r.sevBindings[Notice]   = syslog.LOG_NOTICE
	r.sevBindings[Info]     = syslog.LOG_INFO
	r.sevBindings[Debug]    = syslog.LOG_DEBUG

	r.sevBindings[CustomB1] = syslog.LOG_INFO
	r.sevBindings[CustomB2] = syslog.LOG_INFO

	return r
}

// BindSeverityFlag rebinds severity flag to the new syslog priority code.
func (R *syslogRecorder) BindSeverityFlag(severity MsgFlagT, priority syslog.Priority) error {
	R.Lock(); defer R.Unlock()
	severity = severity &^ severityShadowMask
	if _, exist := R.sevBindings[severity]; !exist {
		return ErrWrongFlagValue
	}

	if ( // syslog.Priority can contains facility codes
		priority != syslog.LOG_EMERG   &&
		priority != syslog.LOG_ALERT   &&
		priority != syslog.LOG_CRIT    &&
		priority != syslog.LOG_ERR     &&
		priority != syslog.LOG_WARNING &&
		priority != syslog.LOG_NOTICE  &&
		priority != syslog.LOG_INFO    &&
		priority != syslog.LOG_DEBUG ) {
		return errWrongPriority
	}	

	R.sevBindings[severity] = priority
	return nil
}

func (R *syslogRecorder) initialise() error {
	R.Lock(); defer R.Unlock()
	//if R.refCounter < 0 { R.refCounter = 0 }
	if R.refCounter == 0 { var err error
		R.logger, err = syslog.New(syslog.LOG_INFO | syslog.LOG_USER, R.prefix)
		if err != nil { return err }
	}
	R.refCounter++
	return nil
}

func (R *syslogRecorder) close() {
	R.Lock(); defer R.Unlock()
	if R.refCounter == 0 { return }
	if R.refCounter == 1 {
		R.logger.Close()
	}
	R.refCounter--
}

// FormatFunc sets custom formatter function for this recorder.
func (R *syslogRecorder) FormatFunc(f FormatFunc) *syslogRecorder {
	R.Lock(); defer R.Unlock()
	R.format = f; return R
}

func (R *syslogRecorder) write(msg LogMsg) error {
	R.Lock(); defer R.Unlock()
	if R.refCounter == 0 { return ErrNotInitialised }
	msgData := msg.content
	if R.format != nil {
		msgData = R.format(&msg)
	}

	sev := msg.flags &^ severityShadowMask
	if priority, exist := R.sevBindings[sev]; exist {
		switch priority {        // WRITE
		case syslog.LOG_EMERG:   R.logger.Emerg(msgData)
		case syslog.LOG_ALERT:   R.logger.Alert(msgData)
		case syslog.LOG_CRIT:    R.logger.Crit(msgData)
		case syslog.LOG_ERR:     R.logger.Err(msgData)
		case syslog.LOG_WARNING: R.logger.Warning(msgData)
		case syslog.LOG_NOTICE:  R.logger.Notice(msgData)
		case syslog.LOG_INFO:    R.logger.Info(msgData)
		case syslog.LOG_DEBUG:   R.logger.Debug(msgData)
		default:
			return internalError(ieUnreachable, "unexpected priority value")
		}
	} else {
		return ErrWrongFlagValue
	}

	return nil
}
