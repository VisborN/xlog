package xlog

import "errors"
import "log/syslog"

type syslogRecorder struct {
	initialised bool
	prefix      string
	format      FormatFunc
	logger      *syslog.Writer

	// says which function to use for each severity
	sevBindings map[uint16]syslog.Priority
}

// NewSyslogRecorder allocates and returns a new syslog recorder.
func NewSyslogRecorder(prefix string) *syslogRecorder {
	r := new(syslogRecorder)
	r.prefix = prefix
	r.sevBindings = make(map[uint16]syslogSeverity)

	// default bindings
	r.sevBindings[Critical] = syslog.LOG_CRIT
	r.sevBindings[Error]    = syslog.LOG_ERR
	r.sevBindings[Warning]  = syslog.LOG_WARNING
	r.sevBindings[Notice]   = syslog.LOG_NOTICE
	r.sevBindings[Info]     = syslog.LOG_INFO
	r.sevBindings[Debug1]   = syslog.LOG_DEBUG
	r.sevBindings[Debug2]   = syslog.LOG_DEBUG
	r.sevBindings[Debug3]   = syslog.LOG_DEBUG

	r.sevBindings[Custom1]  = syslog.LOG_INFO
	r.sevBindings[Custom2]  = syslog.LOG_INFO
	r.sevBindings[Custom3]  = syslog.LOG_INFO
	r.sevBindings[Custom4]  = syslog.LOG_INFO

	return r
}

// BindSeverityFlag rebinds severity flag to the new syslog priority code.
func (R *syslogRecorder) BindSeverityFlag(severity uint16, priority syslog.Priority) error {
	if _, exist := R.sevBindings[severity]; !exist {
		return errors.New("wrong severity value")
	}

	if ( // syslog.Priority can contains facility codes
		priority != syslog.LOG_EMERG   &&
		priority != syslog.LOG_ALERT   &&
		priority != syslog.LOG_CRIT    &&
		priority != syslog.LOG_ERR     &&
		priority != syslog.LOG_WARNING &&
		priority != syslog.LOG_NOTICE  &&
		priority != syslog.LOG_INFO    &&
		priority != syslog.LOG_DEBUG
	) {
		return errors.New("wrong priority value")
	}	

	R.sevBindings[severity] = priority
	return nil
}

func (R *syslogRecorder) initialise() error {
	if R.initialised { return nil }; var err error
	R.logger, err = syslog.New(syslog.LOG_INFO | syslog.LOG_USER, R.prefix)
	if err != nil { return err }
	R.initialised = true
	return nil
}

func (R *syslogRecorder) close() {
	if !R.initialised { return }
	R.initialised = false
	R.logger.Close()
}

// FormatFunc sets custom formatter function for this recorder.
func (R *syslogRecorder) FormatFunc(f FormatFunc) *syslogRecorder {
	R.format = f; return R
}

func (R *syslogRecorder) write(msg LogMsg) error {
	if !R.initialised { return NotInitialised }
	msgData := msg.content
	if R.format != nil {
		msgData = R.format(&msg)
	}

	if priority, exist := R.sevBindings[msg.severity]; exist {
		switch priority {
		case syslog.LOG_EMERG:   R.logger.Emerg(msgData)
		case syslog.LOG_ALERT:   R.logger.Alert(msgData)
		case syslog.LOG_CRIT:    R.logger.Crit(msgData)
		case syslog.LOG_ERR:     R.logger.Err(msgData)
		case syslog.LOG_WARNING: R.logger.Warning(msgData)
		case syslog.LOG_NOTICE:  R.logger.Notice(msgData)
		case syslog.LOG_INFO:    R.logger.Info(msgData)
		case syslog.LOG_DEBUG:   R.logger.Debug(msgData)
		default:
			errors.New("unexpected priority value") // internal
		}
	} else {
		errors.New("wrong severity flag")
	}

	return nil
}
