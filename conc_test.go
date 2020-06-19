package xlog

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/rs/xid"
)

type sharedAction struct {
	name string
	do   func() bool
}

type sharedActionCaller chan sharedAction

func (c sharedActionCaller) Listen() {
	for action := range c {
		if !action.do() {
			fmt.Printf("@ shared action '%s' failed\n", action.name)
		}
	}
}

func TestLoggerRaces(t *testing.T) {
	logger := NewLogger()
	sc1 := make(sharedActionCaller, 16)
	sc2 := make(sharedActionCaller, 16)
	defer func() { close(sc1) }()
	defer func() { close(sc2) }()
	go sc1.Listen()
	go sc2.Listen()

	cerr := func(e error) bool {
		if e != nil {
			return false
		}
		return true
	}

	sc1 <- sharedAction{"RegisterRecorder 1", func() bool {
		return cerr(logger.RegisterRecorder(RecorderID("r1"), emptyRecorder))
	}}
	sc2 <- sharedAction{"RegisterRecorder 2", func() bool {
		return cerr(logger.RegisterRecorder(RecorderID("r2"), emptyRecorder))
	}}
	time.Sleep(time.Millisecond * 10)

	sc1 <- sharedAction{"DefautlsSet 1", func() bool {
		return cerr(logger.DefaultsSet([]RecorderID{"r1"}))
	}}
	sc2 <- sharedAction{"DefautlsSet 2", func() bool {
		return cerr(logger.DefaultsSet([]RecorderID{"r2"}))
	}}
	time.Sleep(time.Millisecond * 10)

	ff := func() bool { return cerr(logger.UnregisterRecorder(RecorderID("r2"))) }
	sc1 <- sharedAction{"UnregisterRecorder 1", ff}
	sc2 <- sharedAction{"UnregisterRecorder 2", ff}
	time.Sleep(time.Millisecond * 10)
}

func TestLoggerDeadLocks(t *testing.T) {

	t.SkipNow()

	/*
	  NumberOfRecorders    r     defer
	  RegisterRecorder     rw    defer
	  UnregisterRecorder   r+rw  manual
	  Initialise           rw    defer
	  Close                rw    defer
	  DefautlsSet          rw    defer
	  DefaultsAdd          rw    defer
	  DefaultsRemove       rw    defer
	  ChangeSeverityOrder  r+rw  manual
	  SetSeverityMask      rw    defer
	  WriteMsg             r     defer
	*/

	t.Run("UnregisterRecorder", func(t *testing.T) {})

	t.Run("ChangeSeverityOrder", func(t *testing.T) {})
}

// =============================================================================

func TestRaceCondMsg(t *testing.T) {
	logger := NewLogger()
	r1 := NewIoDirectRecorder(os.Stdout, "R1")
	r2 := NewIoDirectRecorder(os.Stdout, "R2")
	var rec1ID RecorderID = RecorderID("rec-1")
	var rec2ID RecorderID = RecorderID("rec-2")
	go r1.Listen()
	go r2.Listen()
	defer func() {
		logger.Close()
		runtime.Gosched()
		r1.Intrf().ChCtl <- SignalStop()
		r2.Intrf().ChCtl <- SignalStop()
	}()

	if err := logger.RegisterRecorder(rec1ID, r1.Intrf()); err != nil {
		t.Fatalf("RegisterRecorder(%s) return error\n%v", rec1ID, err)
	}
	if err := logger.RegisterRecorder(rec2ID, r2.Intrf()); err != nil {
		t.Fatalf("RegisterRecorder(%s) return error\n%v", rec2ID, err)
	}
	if err := logger.Initialise(); err != nil {
		t.Fatalf("Initialise() return error\n%v", err)
	}
	runtime.Gosched()
	runtime.Gosched()

	fmt.Print("start cycle\n")
	for i := 0; i < 3; i++ {
		logger.Write(Info, "message %d", i)
		runtime.Gosched()
	}
	fmt.Print("end\n")
}

// -----------------------------------------------------------------------------

type ciControlSignal string

const ciSigWrite ciControlSignal = "CISIG_WRITE"

type callerInstance struct {
	chCtl  chan ciControlSignal
	logger *Logger
	id     xid.ID
}

func newCallerInstance(logger *Logger, chCtl chan ciControlSignal) *callerInstance {
	ci := callerInstance{}
	ci.id = xid.New()
	ci.logger = logger
	ci.chCtl = chCtl
	return &ci
}

func (I *callerInstance) Listen() {
	for sig := range I.chCtl {
		switch sig {
		case ciSigWrite:
			if I.logger != nil {
				if err := I.logger.Write(Info, "message from {ci:%s}", I.id.String()); err != nil {
					fmt.Printf("[callerInstance] Logger.Write() error\n%v", err)
				}
			} else {
				fmt.Print("[callerInstance] FATAL: logger pointer is nil\n")
			}
		default:
			fmt.Print("[callerInstance] RECV UNKNOWN SIGNAL\n")
		}
	}
}

func TestRaceCondLoggerCalls(t *testing.T) {
	const SleepDelay = time.Millisecond * 50
	logger := NewLogger()
	recorder := NewIoDirectRecorder(os.Stdout)
	var recID RecorderID = RecorderID("rec")
	ciControlChannel := make(chan ciControlSignal, 16)
	ci1 := newCallerInstance(logger, ciControlChannel)
	time.Sleep(time.Second + time.Millisecond*147)
	ci2 := newCallerInstance(logger, ciControlChannel)
	go recorder.Listen()
	go ci1.Listen()
	go ci2.Listen()
	//defer func() { ciControlChannel <- ciSigStop }()
	defer func() { close(ciControlChannel) }()
	defer func() { recorder.Intrf().ChCtl <- SignalStop() }()

	if err := logger.RegisterRecorder(recID, recorder.Intrf()); err != nil {
		t.Fatalf("RegisterRecorder() return error\n%v", err)
	}
	if err := logger.Initialise(); err != nil {
		t.Fatalf("Initialise() return error\n%v", err)
	}
	time.Sleep(SleepDelay)
	defer func() {
		logger.Close()
		time.Sleep(SleepDelay)
	}()

	fmt.Print("start cycle\n")
	for i := 0; i < 5; i++ {
		ciControlChannel <- ciSigWrite
		time.Sleep(SleepDelay)
	}
	fmt.Print("end\n")
}
