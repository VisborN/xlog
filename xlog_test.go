package xlog

import (
	"os"
	"testing"
)

func TestGeneral(t *testing.T) {
	logger := NewLogger()
	logger.RegisterRecorder("syslog", NewSyslogRecorder("xlog-test"))
	logger.RegisterRecorder("stdout", NewIoDirectRecorder(os.Stdout))
	if logFile, err :=os.OpenFile("test-general.log",
		os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0644);
	err == nil {
		logger.RegisterRecorder("golog", NewGologRecorder(logFile, "xlog-test").OnClose(
			func(interface{}){ logFile.Close() }))
	} else { t.Errorf("%s", err.Error()) }
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error())
	} else { defer logger.Close() }

	if err := logger.Write(Error, "error msg"); err != nil {
		t.Errorf("%s", err.Error())
	}
	if err := logger.Write(Info, "info msg"); err != nil {
		t.Errorf("%s", err.Error())
	}
}

func TestMultipleLoggers(t *testing.T) {
	logger1 := NewLogger()
	logger2 := NewLogger()
	syslog := NewSyslogRecorder("xlog-test")
	logger1.RegisterRecorder("syslog", syslog)
	logger2.RegisterRecorder("syslog", syslog)
	logger1.Initialise()
	logger2.Initialise() // not need, btw

	logger1.Write(Info, "msg from logger #1")
	logger2.Write(Info, "msg from logger #2")

	logger1.Close()
	logger2.Close()
}
