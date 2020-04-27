package xlog

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

// TODO: Fatal
// TODO: Initialise() return check (BatchResult)

var dc chan<- debugMessage

func TestMain(m *testing.M) {
	dbgFile, err := os.OpenFile("dbg.outp", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("dbg file open fail: %s", err.Error())
		os.Exit(1)
	}
	d := NewDebugLogger(dbgFile)
	dc = d.Chan() // yeah, it can drops
	go d.Listen()

	r := m.Run()

	runtime.Gosched()
	close(d.Chan())
	runtime.Gosched()
	dbgFile.Close()
	os.Exit(r)
}

// TEMPORARY BOTCH FUNCTION just for the hot fix
func ResetErrChan(chErr <-chan error) {
	fmt.Print("resetting error channel:\n")
	for {
		select {
		case err := <-chErr:
			fmt.Printf("  * %s\n", err.Error())
		default:
			fmt.Print("  no more messages\n")
			return
		}
	}
}

func TestGeneral(t *testing.T) {
	dc <- DbgMsg("--- TestGeneral()")
	logger := NewLogger()
	rec := NewIoDirectRecorder(os.Stdout, nil, dc)
	bundle := rec.GetChannels()
	t.Log("register recorder...")
	if err := logger.RegisterRecorder("direct", bundle); err != nil {
		t.Errorf("recorder register fail: direct")
	}
	t.Log("(num of rec check)")
	if logger.NumberOfRecorders() == 0 {
		t.Errorf("no recorders error")
		return
	}

	t.Log("listen <-")
	go rec.Listen() // <----
	//defer func() { rec.GetChannels().chCtl <- SignalStop }()
	dc <- DbgMsg("goshed")
	runtime.Gosched()
	if !rec.IsListening() {
		t.Errorf("CRITICAL: recorder isn't listening")
		return
	}
	t.Log("initialising logger...")
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error())
		return
	} else {
		t.Log("OK")
		defer logger.Close()
	}

	if err := logger.Write(Error, "error message"); err != nil {
		t.Errorf("%s", err.Error())
	}
	time.Sleep(2 * time.Second) // to check timestamp
	if err := logger.Write(Info, "info message"); err != nil {
		t.Errorf("%s", err.Error())
	}
	time.Sleep(time.Microsecond) // for the correct console output

	// listening stop signal check //
	func() { rec.GetChannels().chCtl <- SignalStop }()
	time.Sleep(time.Second) // to allow VM switch stream
	t.Log("stop signal check...")
	if rec.IsListening() {
		t.Errorf("recorder listener still alive")
	} else {
		t.Log("OK")
	}
}

func TestInitialisation(t *testing.T) {
	dc <- DbgMsg("--- TestInitialisation()")
	dc <- DbgMsg("use debugRecorder{} here")

	logger := NewLogger()
	rec1 := t_newDebugRecorder("DR1", nil)
	rec2 := t_newDebugRecorder("DR2", nil)
	go rec1.Listen()
	go rec2.Listen()
	logger.RegisterRecorder("rec-1", rec1.GetChannels())
	logger.RegisterRecorder("rec-2", rec2.GetChannels())

	displayStates := func() {
		msg := "[info] states: "
		msg += fmt.Sprintf("{ logger=%v, ", logger.initialised)
		msg += fmt.Sprintf("rec1=%v, ", rec1.initialised)
		msg += fmt.Sprintf("rec2=%v }", rec2.initialised)
		t.Log(msg)
	}

	displayErrors := func(msg *string, br BatchResult) {
		*msg += "successfull recorders:\n"
		for _, r := range br.ListOfSuccessful() {
			*msg += fmt.Sprintf("  * %s\n", r)
		}
		*msg += "failed recorders:\n"
		for r, e := range br.Errors() {
			*msg += fmt.Sprintf("  * %s: %s\n", r, e)
		}
	}

	// fail on init check //
	t.Log("--> 1st init call (-)")
	rec2.DbgFailInit = true
	if err := logger.Initialise(); err == nil {
		t.Errorf("[unexpected] debug init successful")
		return
	} else {
		displayStates()
		msg := "debug init failed (OK)\n"
		if br, ok := err.(BatchResult); ok {
			displayErrors(&msg, br)
			if err, exists := br.Errors()["rec-2"]; !exists ||
				len(br.Errors()) != 1 || err != t_errManualInvoked {
				t.Errorf("BatchResult.Errors() wrong value\n%s", err.Error())
				return
			}
			if len(br.ListOfSuccessful()) != 1 || br.ListOfSuccessful()[0] != "rec-1" {
				t.Errorf("BatchResult.ListOfSuccessful: wrong value")
				return
			}
			if logger.initialised == true ||
				rec1.initialised != true || rec2.initialised == true {
				t.Errorf(".initialised: wrong value")
				return
			}
			t.Log(msg[:len(msg)-1])
		} else {
			t.Errorf("unexpected error type")
			return
		}
		t.Logf("OK")
	}

	// successful init //
	t.Log("--> 2nd init call (+)")
	rec2.DbgFailInit = false
	err := logger.Initialise()
	displayStates()
	if err != nil {
		t.Errorf("FAIL: logger should be fully initialised\n%s", err.Error())
		return
	} else {
		t.Log("OK")
	}

	// empty init call check //
	t.Log("-> 3rd init call (0)")
	err = logger.Initialise()
	displayStates()
	if err != nil {
		t.Errorf("FAIL: logger should be fully initialised\n%s", err.Error())
		return
	} else {
		t.Log("OK")
	}

	// check flag reset at new recorder //
	t.Log("check init flag resetting...")
	rec3 := t_newDebugRecorder("DR3", nil)
	logger.RegisterRecorder("rec-3", rec3.GetChannels())
	if logger.initialised == true {
		t.Errorf("FAIL: new recorder has been added but logger still initialised")
	} else {
		t.Log("OK")
	}

	// writing to uninitialised logger //
	t.Log("writing to uninitialised logger...")
	err = logger.Write(Info, "")
	t.Logf("retult: err %v", err)
	// TODO: behaviour
}

func TestRefCounter(t *testing.T) {
	dc <- DbgMsg("--- TestRefCounter()")
	//runtime.Gosched() // we don't use def. rec in prev test

	var testValOutputFlag bool = true
	var testValName string = "reference counter"
	testFunc := func(value, expected int) bool {
		if value != expected {
			t.Errorf("unexpected %s value\ncurrent: %d\nexpected: %d",
				testValName, value, expected)
			return false
		}
		if testValOutputFlag {
			t.Logf("%s value: %d", testValName, value)
		}
		return true
	}

	logger1 := NewLogger()
	logger2 := NewLogger()
	rec := NewIoDirectRecorder(os.Stdout, nil, dc)
	defer func() { runtime.Gosched() }()
	go rec.Listen()
	defer func() {
		dc <- DbgMsg("rec1 defer")
		rec.GetChannels().chCtl <- SignalStop
	}()
	logger1.RegisterRecorder("direct", rec.GetChannels())
	logger2.RegisterRecorder("direct", rec.GetChannels())

	testFunc(rec.refCounter, 0) // test startup counter value

	t.Log("-> 1st initialisation of logger1")
	if err := logger1.Initialise(); err != nil {
		t.Errorf("initialisation error: %s", err.Error())
		return
	}
	runtime.Gosched()
	if !testFunc(rec.refCounter, 1) {
	} else {
		t.Log("OK")
	}

	t.Log("-> initialisation of logger2")
	if err := logger2.Initialise(); err != nil {
		t.Errorf("initialisation error: %s", err.Error())
		return
	}
	runtime.Gosched()
	if !testFunc(rec.refCounter, 2) {
	} else {
		t.Log("OK")
	}

	t.Log("-> 2nd initialisation of logger1")
	if err := logger1.Initialise(); err != nil {
		t.Errorf("initialisation error: %s", err.Error())
		return
	}
	runtime.Gosched()
	if !testFunc(rec.refCounter, 2) {
	} else {
		t.Log("OK")
	}

	// ----------------------------------------

	{ // for additional checks
		rec2 := NewIoDirectRecorder(os.Stdout, nil, dc)
		go rec2.Listen()
		defer func() {
			dc <- DbgMsg("rec2 defer")
			rec2.GetChannels().chCtl <- SignalStop
		}()
		logger2.RegisterRecorder("direct-2", rec2.GetChannels())
		t.Log("(logger 2 additional initialisation)")
		if err := logger2.Initialise(); err != nil {
			t.Errorf("initialisation error: %s", err.Error())
		}
	}

	t.Log("-> logger1: unregister recorder (+0INIT)")
	if err := logger1.UnregisterRecorder("direct"); err != nil {
		t.Errorf("unregistering error: %s", err.Error())
		return
	}
	dc <- DbgMsg("goshed")
	runtime.Gosched()
	if !testFunc(rec.refCounter, 1) {
		t.Logf("<debug data>\nrecorder: %v\nlogger: %v", rec, logger1)
	} else {
		// fully uninitialised check
		if logger1.initialised != false {
			t.Log("ref. counter is ok")
			t.Errorf("logger1 is still initialised (was only one rec.)")
		} else {
			if err := logger1.Write(Info, "shouldn't be visible"); err == nil {
				t.Log("ref. counter is ok")
				t.Log("init check passed")
				t.Errorf("logger1.Write() should return an error\n%v", logger1)
			} else {
				t.Log("OK")
			}
		}
	}

	t.Log("-> logger2: unregister recorder (+1INIT)")
	if err := logger2.UnregisterRecorder(("direct")); err != nil {
		t.Errorf("unregistering error: %s", err.Error())
		return
	}
	dc <- DbgMsg("goshed")
	runtime.Gosched()
	if !testFunc(rec.refCounter, 0) {
		t.Logf("recorder: %v", rec)
	} else {
		// partial unregister init check
		if logger2.initialised != true {
			t.Log("ref. counter is ok")
			t.Errorf("FAIL: wrong logger initialised state after unregistering\n%v", logger2)
		} else {
			if err := logger2.Write(Info, "should be visible"); err != nil {
				t.Log("ref. counter is ok")
				t.Log("init check passed")
				t.Errorf("FAIL: logger2.Write() return an error: %s\n%v", err.Error(), logger2)
			}
			t.Log("OK")
		}
	}

	// check writing with no refs on recorder //
	rec.GetChannels().chMsg <- NewLogMsg().Setf("shouldn't be displayed") // TODO
	select {
	case err, ok := <-rec.GetChannels().chErr:
		if !ok {
			t.Errorf("FATAL: error-channel has been closed")
			return
		}
		t.Logf("OK\nerr: %s", err.Error())
	default:
		//t.Errorf("FAIL: no messages in error-channel")
		t.Log("SHADOW-FAIL: no messages in error-channel <NOT IMPLEMENTED YET>")
	}
}

// =======================================================================

func TestSeverityOrder(t *testing.T) {
	dc <- DbgMsg("--- TestSeverityOrder()")
	ResetErrChan(DefErrChan)

	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("file open fail: %s", err.Error())
		return
	}
	rec := NewIoDirectRecorder(logFile, nil, dc).OnClose(func(interface{}) { logFile.Close() })
	go rec.Listen()
	defer func() { runtime.Gosched() }()
	defer func() { rec.GetChannels().chCtl <- SignalStop }()
	if err := logger.RegisterRecorder("direct", rec.GetChannels()); err != nil {
		t.Errorf("recorder register fail: %s", err.Error())
		return
	}
	dc <- DbgMsg("logger: %v", logger)
	dc <- DbgMsg("recorder: %v", rec)
	if err := logger.Initialise(); err != nil {
		if br, ok := err.(BatchResult); ok {
			msg := br.Error()
			for r, e := range br.Errors() {
				msg += fmt.Sprintf("\n%s: %s", r, e.Error())
			}
			t.Errorf("%s", msg)
			return
		} else {
			t.Errorf("unknown error: %s", err.Error())
			return
		}
	} else {
		defer logger.Close()
	}

	// test default order //
	var testFlags MsgFlagT = Error | Info
	if err := logger.severityProtector(
		logger.severityOrder[RecorderID("direct")], &testFlags); err != nil {
		t.Errorf("FAIL: %s", err.Error())
		return
	}
	if testFlags != Error {
		t.Errorf("severityProtector() fail\nresult:   %012b\nexpected: %012b", testFlags, Error)
	}
	msg := NewLogMsg().SetFlags(Error | Info).Setf("should be an error")
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Logf("write error: %s", err.Error())
	}

	// change the order //
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
	msg.UpdateTime().SetFlags(Error | Info).Setf("should be an info")
	if err := logger.WriteMsg(nil, msg); err != nil {
		t.Logf("write error: %s", err.Error())
	}
}

func TestSeverityMask(t *testing.T) {
	dc <- DbgMsg("--- TestSeverityMask()")
	//ResetErrChan(DefErrChan)

	logger := NewLogger()
	logFile, err := os.OpenFile("test.log", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("file open fail: %s", err.Error())
		return
	}
	rec := NewIoDirectRecorder(logFile, nil, dc).OnClose(func(interface{}) {
		logFile.Close()
	})
	go rec.Listen()
	//defer func() { runtime.Gosched() }()
	defer func() { rec.GetChannels().chCtl <- SignalStop }()
	if err := logger.RegisterRecorder("direct", rec.GetChannels()); err != nil {
		t.Errorf("recorder register fail: %s", err.Error())
		return
	}
	if err := logger.Initialise(); err != nil {
		t.Errorf("%s", err.Error())
		return
	} else {
		defer logger.Close()
	}

	// sev. include option //
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

	logger.Write(Emerg, "should be hidden")
	logger.Write(Alert, "should be hidden")
	logger.Write(Critical, "should be hidden")
	logger.Write(Warning, "should be hidden")

	// sev. exclude option //
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

	logger.Write(Critical, "should be hidden")
	logger.Write(Error, "should be hidden")
}

/*
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
*/

// -----------------------------------------------------------------------------
//                        ***** DEBUG RECORDER *****
// -----------------------------------------------------------------------------

var t_errManualInvoked = fmt.Errorf("manual invoked error")

type t_debugRecorder struct {
	chCtl chan ControlSignal
	chMsg chan *LogMsg
	chErr chan error

	initialised  bool
	refCounter   int
	DbgFailInit  bool
	DbgFailWrite bool
	DbgOutput    *string
	iid          string
}

func t_newDebugRecorder(iid string, outp *string) *t_debugRecorder {
	r := new(t_debugRecorder)
	r.chCtl = make(chan ControlSignal, 5)
	r.chMsg = make(chan *LogMsg, 5)
	r.chErr = make(chan error)
	r.DbgOutput = outp
	r.iid = iid
	return r
}

func (R *t_debugRecorder) GetChannels() ChanBundle {
	return ChanBundle{R.chCtl, R.chMsg, R.chErr}
}

func (R *t_debugRecorder) Listen() {
	for {
		select {
		case msg := <-R.chCtl:
			switch msg {
			case SignalInit:
				if R.DbgFailInit {
					R.chErr <- t_errManualInvoked
					continue
				}
				R.initialised = true
				R.refCounter++
				R.chErr <- nil
			case SignalClose:
				if R.refCounter > 0 {
					R.refCounter--
					if R.refCounter == 0 {
						R.initialised = false
					}
				}
			}
		case msg := <-R.chMsg:
			err := R.write(*msg)
			R.chErr <- err
		}
	}
}

func (R *t_debugRecorder) write(msg LogMsg) error {
	if R.DbgOutput != nil {
		*R.DbgOutput = fmt.Sprintf("[%s] initialised=%v  refCounter=%d",
			R.iid, R.initialised, R.refCounter)
	}
	if !R.initialised {
		return ErrNotInitialised
	}
	if R.DbgFailWrite {
		return t_errManualInvoked
	}
	return nil
}
