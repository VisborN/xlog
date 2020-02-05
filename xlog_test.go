package xlog

import "os"
import "time"
import "testing"

func TestGeneral(t *testing.T) {
	logger := NewLogger()	
	logFile, err := os.OpenFile("test.log", os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0644)
	if err != nil { t.Errorf("file open fail: %s", err.Error()); return }
	logFile.Write([]byte("------------------------------------------\n"))
	if !logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile).OnClose(
		func(interface{}){ logFile.Close() },
	)) { t.Errorf("recorder register fail: direct") }
	if !logger.RegisterRecorder("syslog", NewSyslogRecorder("xlog-test")) {
		t.Errorf("recorder register fail: syslog")
	}

	if logger.NumberOfRecorders() == 0 { return }
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error()); return
	} else { defer logger.Close() }

	if err := logger.Write(Error, "1 error message"); err != nil {
		t.Errorf("%s", err.Error())
	}; time.Sleep(2 * time.Second)
	if err := logger.Write(Info, "1 info message"); err != nil {
		t.Errorf("%s", err.Error())
	}
	if err := logger.WriteMsg([]RecorderID{"direct"},
		Message("1 only for direct recorder"));
	err != nil { t.Errorf("%s", err.Error()) }
}

func TestLogMsg(t *testing.T) {
	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND | os.O_WRONLY, 0644)
	if err != nil { t.Errorf("file open fail: %s", err.Error()); return }
	if !logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile,).OnClose(
		func(interface{}){ logFile.Close() },
	)) { t.Errorf("recorder register fail: direct"); return }
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error()); return
	} else { defer logger.Close() }

	msg := NewLogMsg().Severity(Critical)
	msg.Setf("2 the message header")
	msg.Addf("\nnew line (manual)")
	msg.Addf_ln("new line (auto)")
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Errorf("%s", err.Error())
	}
	if msg.GetSeverity() != Critical || msg.GetContent() !=
		"2 the message header\nnew line (manual)\nnew line (auto)" {
		t.Errorf("error, unexpected message data\n%v", msg)
	}

	msg = NewLogMsg().Severity(Debug1)
	msg.Addf("2 original message\nsecond line")
	originalTime := msg.GetTime()
	msg.Setf("2 overwritten message").Severity(Info).UpdateTime()
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Errorf("%s", err.Error())
	}
	if msg.GetSeverity() != Info { t.Errorf("error, unexpected message severity") }
	if msg.GetTime() == originalTime { t.Errorf("error, unexpected message time value") }
	if msg.GetContent() != "2 overwritten message" { t.Errorf("error, unexpected message data") }
}

func TestSeverityOrder(t *testing.T) {
	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND | os.O_WRONLY, 0644)
	if err != nil { t.Errorf("file open fail: %s", err.Error()); return }
	if !logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile,).OnClose(
		func(interface{}){ logFile.Close() },
	)) { t.Errorf("recorder register fail: direct"); return }
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error()); return
	} else { defer logger.Close() }

	if sev := logger.severityProtector(
		logger.severityOrder[RecorderID("direct")], Error | Info); sev != Error {
		t.Errorf("severityProtector() fail\nresult:   %012b\nexpected: %012b", sev, Error)
	}
	msg := NewLogMsg().Severity(Error | Info).Setf("3 should be error")
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Logf("write error: %s", err.Error())
	}

	// change it
	if err := logger.ChangeSeverityOrder("direct", Info, Before, Error); err != nil {
		t.Errorf("%s", err.Error()); return
	}

	if sev := logger.severityProtector(
		logger.severityOrder[RecorderID("direct")], Error | Info); sev != Info {
		t.Errorf("severityProtector() fail\nresult:   %012b\nexpected: %012b", sev, Info)
	}
	msg.UpdateTime().Severity(Error | Info).Setf("3 should be info")
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Logf("write error: %s", err.Error())
	}
}

func TestSeverityMask(t *testing.T) {
	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND | os.O_WRONLY, 0644)
	if err != nil { t.Errorf("file open fail: %s", err.Error()); return }
	if !logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile,).OnClose(
		func(interface{}){ logFile.Close() },
	)) { t.Errorf("recorder register fail: direct"); return }
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error()); return
	} else { defer logger.Close() }

	// > include
	if err := logger.SetSeverityMask("direct", Error | Notice | Info | Debug1); err != nil {
		t.Errorf("%s", err.Error()); return
	} else {
		t.Logf("sev mask: %012b (%v)",
			logger.severityMasks["direct"],
			logger.severityMasks["direct"])
	}
	if v:= logger.severityMasks["direct"]; v != 0x3A {
		t.Errorf("unexpected mask value\ncurrent:  %012b\nexpected: %012b", v, 0x3A)
	}

	logger.Write(Critical, "4 should be hidden")
	//logger.Write(Error,    "4 should be visible")
	logger.Write(Warning,  "4 should be hidden")
	//logger.Write(Notice,   "4 should be visible")
	//logger.Write(Info,     "4 should be visible")
	//logger.Write(Debug1,   "4 should be visible")
	logger.Write(Debug2,   "4 should be hidden")
	logger.Write(Debug3,   "4 should be hidden")

	// > exclude
	if err := logger.SetSeverityMask("direct", SeverityAll &^ Critical &^ Error); err != nil {
		t.Errorf("%s", err.Error()); return
	} else {
		t.Logf("sev mask: %012b (%v)",
			logger.severityMasks["direct"],
			logger.severityMasks["direct"])	
	}
	if v := logger.severityMasks["direct"]; v != 0xFFC {
		t.Errorf("unexpected mask value\ncurrent:  %012b\nexpected: %012b", v, 0xFFC)
	}

	logger.Write(Critical, "4 should be hidden")
	logger.Write(Error,    "4 should be hidden")
	//logger.Write(Warning,  "4 should be visible")
	//logger.Write(Notice,   "4 should be visible")
	//logger.Write(Info,     "4 should be visible")
	//logger.Write(Debug1,   "4 should be visible")
	//logger.Write(Debug2,   "4 should be visible")
	//logger.Write(Debug3,   "4 should be visible")
}

var testValueName string
var testValueOutpFlag bool
func testValue(t *testing.T, value int, expected int) bool { // utility function
	if value != expected {
		t.Errorf("unexpected %s value\ncurrent:  %d\nexpected: %d",
			testValueName, value, expected)
		return false
	}
	if testValueOutpFlag {
		t.Logf("%s value: %d", testValueName, value)
	}
	return true
}

func TestRefCounters(t *testing.T) {
	logger1 := NewLogger()
	logger2 := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND | os.O_WRONLY, 0644)
	if err != nil { t.Errorf("file open fail: %s", err.Error()); return }
	recorder := NewIoDirectRecorder(logFile)
	logger1.RegisterRecorder("direct", recorder)
	logger2.RegisterRecorder("direct", recorder)
	testValueName = "reference counter"

	if !testValue(t, recorder.refCounter, 0) { return }
	logger1.Initialise()
	if !testValue(t, recorder.refCounter, 1) { return }
	logger2.Initialise()
	if !testValue(t, recorder.refCounter, 2) { return }


	// trying second initialisation call
	if err := logger2.Initialise(); err != nil {
		t.Logf("initialisation error: %s", err.Error())
		return
	}
	if !testValue(t, recorder.refCounter, 2) { return }

	// ----------

	if err := logger1.Write(Info, "5 logger 1 message"); err != nil {
		t.Errorf("logger 1 write error: %s", err.Error())
	}
	if err := logger2.Write(Info, "5 logger 2 message"); err != nil {
		t.Errorf("logger 2 write error: %s", err.Error())
	}

	// ----------

	logger1.Close()
	if !testValue(t, recorder.refCounter, 1) { return }	

	// logger.WriteMsg() protection test
	if err := logger1.Write(Info, "5 this write call should be failed"); err == nil {
		t.Errorf("FAIL (1), this write call should return error\n"+
			"we shouldn't be able to write to the closed logger")
		return
	}

	if err := logger2.Write(Info, "5 should be visible"); err != nil {
		t.Errorf("write error after 1st close: %s", err.Error())
		return
	}	

	logger2.Close()
	if !testValue(t, recorder.refCounter, 0) { return }
	if err := logger2.Write(Info, "5 this write call should be failed"); err == nil {
		t.Errorf("FAIL (2), this write call should return error")
		return
	}

	// trying second close call
	logger2.Close()
	if !testValue(t, recorder.refCounter, 0) { return }
}
