package xlog

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/rs/xid"
)

var dc chan<- debugMessage

var emptyRecorder RecorderInterface

func TestMain(m *testing.M) {
	emptyRecorder = RecorderInterface{}
	emptyRecorder.ChCtl = make(chan<- controlSignal)
	emptyRecorder.ChMsg = make(chan<- LogMsg)

	dbgFile, err := os.OpenFile("dbg.outp", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("dbg file open fail: %s", err.Error())
		os.Exit(1)
	}
	d := NewDebugLogger(dbgFile)
	dc = d.Chan() // yeah, it can drops
	go d.Listen()

	r := m.Run()

	time.Sleep(time.Millisecond * 5)
	//d.Close() // dodge an error
	dbgFile.Close()
	os.Exit(r)
}

func TestGeneral(t *testing.T) {
	// we have several goroutines, Gosched will be not effective here
	const SleepDelay = time.Millisecond * 50

	file, err := os.OpenFile("test.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile() error\n%s", err.Error())
	}

	l := NewLogger()
	r1 := SpawnIoDirectRecorder(os.Stdout)
	r2 := SpawnIoDirectRecorder(file)
	defer func() { r1.Intrf().ChCtl <- SignalStop() }()
	defer func() { r2.Intrf().ChCtl <- SignalStop() }()
	defer func() { l.Close(); runtime.Gosched() }()
	var rec1 RecorderID = RecorderID("rec-1")
	var rec2 RecorderID = RecorderID("rec-2")
	r1.Intrf().ChCtl <- SignalSetDbgChan(dc)
	r2.Intrf().ChCtl <- SignalSetDbgChan(dc)
	time.Sleep(SleepDelay)

	r1.OnClose(func(interface{}) {
		fmt.Println("CLOSER FUNC")
		file.Close()
	})
	if r1.closer == nil {
		t.Errorf("[%s] closer func is nil, wrong value", rec1)
	}

	dc <- DbgMsg(xid.NilID(), "--- %s", t.Name())

	if err := l.RegisterRecorder(rec1, r1.Intrf()); err != nil {
		t.Fatalf("[%s] RegisterRecorder return error\n%s", rec1, err.Error())
	}
	if err := l.RegisterRecorder(rec2, r2.Intrf()); err != nil {
		t.Fatalf("[%s] RegisterRecorder return error\n%s", rec2, err.Error())
	}
	if err := l.Initialise(); err != nil {
		t.Fatalf("Initialise return error\n%s", err.Error())
	}
	time.Sleep(SleepDelay)

	var tSevMask MsgFlagT = SeverityAll &^ Critical
	if err := l.SetSeverityMask(rec2, tSevMask); err != nil {
		t.Fatalf("SetSeverityMask() return error\n%s", err.Error())
	} else {
		if v, exist := l.severityMasks[rec2]; !exist {
			t.Fatalf("[CRIT] .severityMasks[%s] doesn't exist", rec2)
		} else {
			if v != tSevMask {
				t.Fatalf(".severityMasks[%s] wrong value", rec2)
			}
		}
	}

	t.Log("sending 3 messages...")
	msg := NewLogMsg().SetFlags(Info)
	msg.Setf("this is an regular message for both recorders | c:1")
	if err := l.WriteMsg(nil, msg); err != nil {
		t.Errorf("WriteMsg return error\nerr: %s\nmsg: %s", err.Error(), msg.content)
	}
	time.Sleep(SleepDelay)

	msg.SetFlags(Critical)
	msg.Setf("this message should be sent only to the stdout | c:1,2")
	if err := l.WriteMsg(nil, msg); err != nil {
		t.Errorf("WriteMsg return error\nerr: %s\nmsg: %s", err.Error(), msg.content)
	}
	time.Sleep(SleepDelay)

	msg.SetFlags(Info | Notice).Addf(",3")
	if err := l.WriteMsg([]RecorderID{rec1}, msg); err != nil {
		t.Errorf("WriteMsg return error\nerr: %s\nmsg: %s", err.Error(), msg.content)
	}
	time.Sleep(SleepDelay)
}

func TestSyslogRec(t *testing.T) {
	t.SkipNow()
}
