package xlog

import "log/syslog"

type syslogRecorder struct {
	initialised bool
	prefix      string
	format      FormatFunc
	logger      *syslog.Writer
}

// NewSyslogRecorder allocates  and returns a new syslog recorder.
func NewSyslogRecorder(prefix string) *syslogRecorder {
	r := new(syslogRecorder)
	r.prefix = prefix
	return r
}

func (R *syslogRecorder) initialise() error {
	if R.initialised { return nil }; var err error
	R.logger, err = syslog.New(syslog.LOG_INFO | syslog.LOG_USER, R.prefix)
	if err != nil { return err }
	R.initialised = true
	return nil
}

func (R *syslogRecorder) close() {
	R.initialised = false
	R.logger.Close()
}

// FormatFunc sets custom formatter function for this recorder.
func (R *syslogRecorder) FormatFunc(f FormatFunc) *syslogRecorder {
	R.format = f; return R
}

// this function can invoke panic on critical error (unreachable)
func (R *syslogRecorder) write(msg logMsg) error {
	if !R.initialised { return NotInitialised }
	msgData := msg.content
	if R.format != nil {
		msgData = R.format(&msg)
	}
	switch msg.severity {
	case Critical: R.logger.Crit(msgData)
	case Error:    R.logger.Err(msgData)
	case Warning:  R.logger.Warning(msgData)
	case Notice:   R.logger.Notice(msgData)
	case Info:     R.logger.Info(msgData)
	case Debug1:   R.logger.Debug("@D1 " + msgData)
	case Debug2:   R.logger.Debug("@D2 " + msgData)
	case Debug3:   R.logger.Debug("@D3 " + msgData)
	default: // unreachable, upper call-chain checks
		panic("xlog: unexpected severity value")
	}
	return nil
}
