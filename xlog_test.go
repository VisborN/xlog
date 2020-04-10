package xlog

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestGeneral(t *testing.T) {
	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("file open fail: %s", err.Error())
		return
	}
	logFile.Write([]byte("------------------------------------------\n"))
	if err := logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile).OnClose(
		func(interface{}) { logFile.Close() },
	)); err != nil {
		t.Errorf("recorder register fail: direct")
	}
	if err := logger.RegisterRecorder("syslog", NewSyslogRecorder("xlog-test")); err != nil {
		t.Errorf("recorder register fail: syslog")
	}

	if logger.NumberOfRecorders() == 0 {
		return
	}
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error())
		return
	} else {
		defer logger.Close()
	}

	if err := logger.Write(Error, "1 error message"); err != nil {
		t.Errorf("%s", err.Error())
	}
	time.Sleep(2 * time.Second)
	if err := logger.Write(Info, "1 info message"); err != nil {
		t.Errorf("%s", err.Error())
	}
	if err := logger.WriteMsg([]RecorderID{"direct"},
		Message("1 only for direct recorder")); err != nil {
		t.Errorf("%s", err.Error())
		if br, ok := err.(BatchResult); ok {
			msg := fmt.Sprintf("error msg: %s\n", br.Error())
			for recID, e := range br.Errors() {
				msg += fmt.Sprintf("%s: %s\n", recID, e.Error())
			}
			t.Log(msg[:len(msg)-1])
		}
	}
}

func TestLogMsg(t *testing.T) {
	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("file open fail: %s", err.Error())
		return
	}
	if err := logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile).OnClose(
		func(interface{}) { logFile.Close() },
	)); err != nil {
		t.Errorf("recorder register fail: direct")
		return
	}
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error())
		return
	} else {
		defer logger.Close()
	}

	msg := NewLogMsg().SetFlags(Critical)
	msg.Setf("2 the message header")
	msg.Addf("\nnew line (manual)")
	msg.Addf_ln("new line (auto)")
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Errorf("%s", err.Error())
	}
	if (msg.GetFlags()&^SeverityShadowMask) != Critical || msg.GetContent() !=
		"2 the message header\nnew line (manual)\nnew line (auto)" {
		t.Errorf("error, unexpected message data\n%v", msg)
	}

	msg = NewLogMsg().SetFlags(Debug)
	msg.Addf("2 original message\nsecond line")
	originalTime := msg.GetTime()
	msg.Setf("2 overwritten message").SetFlags(Info).UpdateTime()
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Errorf("%s", err.Error())
	}
	if (msg.GetFlags() &^ SeverityShadowMask) != Info {
		t.Errorf("error, unexpected message severity")
	}
	if msg.GetTime() == originalTime {
		t.Errorf("error, unexpected message time value")
	}
	if msg.GetContent() != "2 overwritten message" {
		t.Errorf("error, unexpected message data")
	}
}

func TestSeverityOrder(t *testing.T) {
	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("file open fail: %s", err.Error())
		return
	}
	if err := logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile).OnClose(
		func(interface{}) { logFile.Close() },
	)); err != nil {
		t.Errorf("recorder register fail: direct")
		return
	}
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error())
		return
	} else {
		defer logger.Close()
	}

	var testFlags MsgFlagT = Error | Info
	if err := logger.severityProtector(
		logger.severityOrder[RecorderID("direct")], &testFlags); err != nil {
		t.Errorf("FAIL: %s", err.Error())
		return
	}
	if testFlags != Error {
		t.Errorf("severityProtector() fail\nresult:   %012b\nexpected: %012b", testFlags, Error)
	}
	msg := NewLogMsg().SetFlags(Error | Info).Setf("3 should be error")
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Logf("write error: %s", err.Error())
	}

	// change it
	if err := logger.ChangeSeverityOrder("direct", Info, Before, Error); err != nil {
		t.Errorf("%s", err.Error())
		return
	}

	testFlags = Error | Info
	if err := logger.severityProtector(
		logger.severityOrder[RecorderID("direct")], &testFlags); err != nil {
		t.Errorf("FAIL: %s", err.Error())
		return
	}
	if testFlags != Info {
		t.Errorf("severityProtector() fail\nresult:   %012b\nexpected: %012b", testFlags, Info)
	}
	msg.UpdateTime().SetFlags(Error | Info).Setf("3 should be info")
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Logf("write error: %s", err.Error())
	}
}

func TestSeverityMask(t *testing.T) {
	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("file open fail: %s", err.Error())
		return
	}
	if err := logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile).OnClose(
		func(interface{}) { logFile.Close() },
	)); err != nil {
		t.Errorf("recorder register fail: direct")
		return
	}
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error())
		return
	} else {
		defer logger.Close()
	}

	// > include
	if err := logger.SetSeverityMask("direct", Error|Notice|Info|Debug); err != nil {
		t.Errorf("%s", err.Error())
		return
	} else {
		t.Logf("sev mask: %016b (%v)",
			logger.severityMasks["direct"],
			logger.severityMasks["direct"])
	}
	if v := logger.severityMasks["direct"]; v != 0xE8 {
		t.Errorf("unexpected mask value\ncurrent:  %016b\nexpected: %016b", v, 0x3A)
	}

	logger.Write(Emerg, "4 should be hidden")
	logger.Write(Alert, "4 should be hidden")
	logger.Write(Critical, "4 should be hidden")
	//logger.Write(Error,    "4 should be visible")
	logger.Write(Warning, "4 should be hidden")
	//logger.Write(Notice,   "4 should be visible")
	//logger.Write(Info,     "4 should be visible")
	//logger.Write(Debug,   "4 should be visible")

	// > exclude
	if err := logger.SetSeverityMask("direct", SeverityAll&^Critical&^Error); err != nil {
		t.Errorf("%s", err.Error())
		return
	} else {
		t.Logf("sev mask: %016b (%v)",
			logger.severityMasks["direct"],
			logger.severityMasks["direct"])
	}
	if v := logger.severityMasks["direct"]; v != 0x30F3 {
		t.Errorf("unexpected mask value\ncurrent:  %016b\nexpected: %016b", v, 0xFFC)
	}

	//logger.Write(Emerg, "4 should be visible")
	//logger.Write(Alert, "4 should be visible")
	logger.Write(Critical, "4 should be hidden")
	logger.Write(Error, "4 should be hidden")
	//logger.Write(Warning,  "4 should be visible")
	//logger.Write(Notice,   "4 should be visible")
	//logger.Write(Info,     "4 should be visible")
	//logger.Write(Debug1,   "4 should be visible")
	//logger.Write(Debug2,   "4 should be visible")
	//logger.Write(Debug3,   "4 should be visible")
}

func TestInitialisation(t *testing.T) {
	var dbgOutp string
	logger := NewLogger()
	dbgRecorder1 := t_newDebugRecorder("DR1", &dbgOutp)
	dbgRecorder2 := t_newDebugRecorder("DR2", &dbgOutp)
	dbgRecorder3 := t_newDebugRecorder("DR3", &dbgOutp)
	logger.RegisterRecorder("debug-1", dbgRecorder1)
	logger.RegisterRecorder("debug-2", dbgRecorder2)

	fShowData := func() {
		msg := "<show info>\n"
		msg += fmt.Sprintf("logger: initialised=%v\n", logger.initialised)
		for recID, state := range logger.recordersState {
			msg += fmt.Sprintf("  %-10s : %v\n", recID, state)
		}
		msg += "recorders data:\n"
		for _, rec := range logger.recorders {
			rec.write(*NewLogMsg())
			msg += fmt.Sprintf("  %s\n", dbgOutp)
			dbgOutp = ""
		}
		t.Log(msg[:len(msg)-1])
	}

	t.Log("--> 1st initialisation call")
	dbgRecorder2.DbgFailInit = true
	err := logger.Initialise()
	if err == nil {
		t.Errorf("[unexpected behaviour] initialisation success")
	} else {
		msg := "[OK] debug initialisation failed\n"
		if br, ok := err.(BatchResult); ok {
			msg += "---successful---\n"
			for _, r := range br.ListOfSuccessful() {
				msg += fmt.Sprintf("%s\n", r)
			}
			msg += "---failed---\n"
			for r, e := range br.Errors() {
				msg += fmt.Sprintf("%s: %s\n", r, e)
			}
		} else {
			t.Errorf("unexpected error type")
		}
		t.Log(msg[:len(msg)-1])
	}
	fShowData()

	t.Log("--> 2nd initialisation call")
	//dbgRecorder1.DbgFailInit = true
	dbgRecorder2.DbgFailInit = false
	err = logger.Initialise()
	t.Logf("result: %v", err)
	fShowData()

	t.Log("--> 3rd initialisation call")
	err = logger.Initialise()
	t.Logf("result: %v", err)
	fShowData()

	// reset flag at new recorder
	if err := logger.RegisterRecorder("debug-3", dbgRecorder3); err != nil {
		t.Errorf("rr initialisation error: %s", err.Error())
	}
	if logger.initialised == true {
		t.Errorf("FAIL: logger still initialised after adding new recorder")
	}
}

func TestUnregistering(t *testing.T) {
	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("file open fail: %s", err.Error())
		return
	}
	logFile.Write([]byte("------------------------------------------\n"))
	if err := logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile).OnClose(
		func(interface{}) {
			logFile.Close()
			//fmt.Printf("<FCLOSED>\n")
		},
	)); err != nil {
		t.Errorf("recorder register fail: direct")
	}
	if err := logger.RegisterRecorder("syslog", NewSyslogRecorder("xlog-test")); err != nil {
		t.Errorf("recorder register fail: syslog")
	}
	if err := logger.Initialise(); err != nil {
		t.Errorf("initialisation fail: %s", err.Error())
		return
	}

	t.Logf("%v", logger)
	if err := logger.UnregisterRecorder("direct"); err != nil {
		t.Errorf("unregister fail: %s", err.Error())
	}
	t.Logf("%v", logger)
}

func TestStackTrace(t *testing.T) {
	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("file open fail: %s", err.Error())
		return
	}
	if err := logger.RegisterRecorder("direct", NewIoDirectRecorder(logFile).OnClose(
		func(interface{}) { logFile.Close() },
	)); err != nil {
		t.Errorf("recorder register fail: direct")
		return
	}
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error())
		return
	} else {
		defer logger.Close()
	}

	if err := logger.Write(StackTrace, "5 msg with stack trace (full)"); err != nil {
		t.Errorf("write error: %s", err.Error())
	}

	if err := logger.Write(StackTraceShort|StackTrace, "5 msg with stack trace (short)"); err != nil {
		t.Errorf("write error: %s", err.Error())
	}
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
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("file open fail: %s", err.Error())
		return
	}
	recorder := NewIoDirectRecorder(logFile)
	logger1.RegisterRecorder("direct", recorder)
	logger2.RegisterRecorder("direct", recorder)
	testValueName = "reference counter"

	t.Log("--> 1st initialisation of logger1")
	t.Log("--> 1st initialisation of logger2")
	if !testValue(t, recorder.refCounter, 0) {
		return
	}
	if err := logger1.Initialise(); err != nil {
		t.Errorf("initialisation error: %s", err.Error())
	}
	if !testValue(t, recorder.refCounter, 1) {
		return
	}
	if err := logger2.Initialise(); err != nil {
		t.Errorf("initialisation error: %s", err.Error())
	}
	if !testValue(t, recorder.refCounter, 2) {
		return
	}

	// trying second initialisation call
	t.Log("--> 2nd initialisation of logger2")
	if err := logger2.Initialise(); err != nil {
		t.Logf("initialisation error: %s", err.Error())
		return
	}
	if !testValue(t, recorder.refCounter, 2) {
		return
	}

	// ----------

	t.Log("--> writing to loggers")
	if err := logger1.Write(Info, "6 logger 1 message"); err != nil {
		t.Errorf("logger 1 write error: %s", err.Error())
	}
	if err := logger2.Write(Info, "6 logger 2 message"); err != nil {
		t.Errorf("logger 2 write error: %s", err.Error())
	}

	// ----------

	t.Log("--> closing logger1")
	logger1.Close()
	if !testValue(t, recorder.refCounter, 1) {
		return
	}

	// logger.WriteMsg() protection test
	if err := logger1.Write(Info, "6 this write call should be failed"); err == nil {
		t.Errorf("FAIL (1), this write call should return error\n" +
			"we shouldn't be able to write to the closed logger")
		return
	}

	if err := logger2.Write(Info, "6 should be visible"); err != nil {
		t.Errorf("write error after 1st close: %s", err.Error())
		return
	}

	t.Log("--> 1st closing of logger2")
	logger2.Close()
	if !testValue(t, recorder.refCounter, 0) {
		return
	}
	if err := logger2.Write(Info, "5 this write call should be failed"); err == nil {
		t.Errorf("FAIL (2), this write call should return error")
		return
	}

	// trying second close call
	t.Log("--> 2st closing of logger2")
	logger2.Close()
	if !testValue(t, recorder.refCounter, 0) {
		return
	}
}

// -----------------------------------------------------------------------------
//                        ***** DEBUG RECORDER *****
// -----------------------------------------------------------------------------

type debugRecorder struct { // ioDirectRecorder behaviour
	initialised  bool
	refCounter   int
	DbgFailInit  bool
	DbgFailWrite bool
	DbgOutput    *string
	iid          string
}

func t_newDebugRecorder(iid string, outp *string) *debugRecorder {
	r := new(debugRecorder)
	r.DbgOutput = outp
	r.iid = iid
	return r
}

func (R *debugRecorder) initialise() error {
	if R.DbgFailInit {
		return fmt.Errorf("debug error")
	}
	R.initialised = true
	R.refCounter++
	return nil
}

func (R *debugRecorder) close() {
	//if !R.initialised { return }
	if R.refCounter <= 0 {
		return
	}
	if R.refCounter == 1 {
		R.initialised = false
	}
	R.refCounter--
}

func (R *debugRecorder) isInitialised() bool {
	return R.initialised
}

func (R *debugRecorder) write(msg LogMsg) error {
	if R.DbgOutput != nil {
		*R.DbgOutput = fmt.Sprintf(
			"[%s] initialised=%v  refCounter=%d",
			R.iid, R.initialised, R.refCounter)
	}
	if !R.initialised {
		return ErrNotInitialised
	}
	if R.DbgFailWrite {
		return fmt.Errorf("debug error")
	}
	return nil
}
