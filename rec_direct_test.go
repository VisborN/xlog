package xlog

import (
	"fmt"
	"testing"
	"time"

	"github.com/rs/xid"
)

type VoidWriter struct {
	prefail  bool_s
	callback func(bool)
}

func NewVoidWriter() *VoidWriter {
	vw := VoidWriter{}
	vw.prefail.Set(false)
	return &vw
}

func (w *VoidWriter) Write(p []byte) (n int, err error) {
	errToBool := func(e error) bool {
		if e != nil {
			return false
		}
		return true
	}
	if w.prefail.Get() {
		err = fmt.Errorf("manual caused error")
	}
	if w.callback != nil {
		w.callback(errToBool(err))
	}
	//fmt.Printf("[VoidWriter] Write(): prefail is %v, error = %v\n", w.prefail.Get(), err)
	return -1, err
}

func TestListenFunc(t *testing.T) {
	// too many goroutines, Gosched will be not effective
	const SleepDelay = time.Millisecond * 10

	writer := NewVoidWriter()
	r := NewIoDirectRecorder(writer)

	dc <- DbgMsg(xid.NilID(), "----- %s", t.Name())

	if !t.Run("wrapper", func(t *testing.T) {
		go r.Listen()
		defer func() { r.Intrf().ChCtl <- SignalStop() }()
		var closerExecuted bool
		r.OnClose(func(interface{}) {
			if closerExecuted {
				t.Fatalf("closer func already executed")
			}
			closerExecuted = true
		})

		// ----------------------------------------

		chWriteErr := make(chan error)
		defer func() { close(chWriteErr) }()
		var writeErrorReceived bool
		go func() {
			for err := range chWriteErr {
				if err == nil {
					fmt.Print("[writeErrorListener] nil has been received (unexpected behaviour)\n")
				} else {
					//fmt.Print("[writeErrorListener] error received, OK\n")
					writeErrorReceived = true
				}
			}
		}()
		r.Intrf().ChCtl <- SignalSetErrChan(chWriteErr)
		time.Sleep(SleepDelay)
		if r.chErr == nil {
			t.Fatalf(".chErr is nil")
		}

		r.Intrf().ChCtl <- SignalSetDbgChan(dc)
		time.Sleep(SleepDelay)
		if r.chDbg == nil {
			t.Fatalf(".chDbg is nil")
		}

		chInitErr := make(chan error)
		r.Intrf().ChCtl <- SignalInit(chInitErr)
		if res := <-chInitErr; res != nil {
			t.Fatalf("[1] initialisation failed")
		}
		if r.refCounter != 1 {
			t.Errorf("wrong .refCounter value (%d/1)", r.refCounter)
		}
		r.Intrf().ChCtl <- SignalInit(chInitErr)
		if res := <-chInitErr; res != nil {
			t.Fatalf("[2] initialisation failed")
		}
		if r.refCounter != 2 {
			t.Errorf("wrong .refCounter value (%d/2)", r.refCounter)
		}
		close(chInitErr)

		// ----------------------------------------

		type WriteOpStatus struct {
			success  bool
			executed bool
		}
		var wstatus WriteOpStatus
		writer.callback = func(b bool) {
			wstatus.executed = true
			wstatus.success = b
		}

		r.Intrf().ChMsg <- *NewLogMsg()
		time.Sleep(SleepDelay)
		if !wstatus.executed {
			t.Errorf("[1] write action is not executed")
		} else if !wstatus.success {
			t.Errorf("[1] unexpected result of write operation (%v)", wstatus.success)
		}

		writer.prefail.Set(true)
		wstatus.executed = false
		r.Intrf().ChMsg <- *NewLogMsg()
		time.Sleep(SleepDelay)
		if !wstatus.executed {
			t.Errorf("[2] write action is not executed")
		} else if wstatus.success {
			t.Errorf("[2] unexpected result of write operation (%v)", wstatus.success)
		}
		//time.Sleep(SleepDelay)
		if !writeErrorReceived {
			t.Errorf("[2] write error channel didn't receive an error signal")
		}

		// ----------------------------------------

		r.Intrf().ChCtl <- SignalDropErrChan()
		time.Sleep(SleepDelay)
		if r.chErr != nil {
			t.Errorf(".chErr != nil (should be dropped)")
		}
		r.Intrf().ChCtl <- SignalDropDbgChan()
		time.Sleep(SleepDelay)
		if r.chDbg != nil {
			t.Errorf(".chDbg != nil (should be dropped)")
		}

		r.Intrf().ChCtl <- SignalClose()
		time.Sleep(SleepDelay)
		if r.refCounter != 1 {
			t.Errorf("wrong .refCounter value (%d/1)", r.refCounter)
		}
		r.Intrf().ChCtl <- SignalClose()
		time.Sleep(SleepDelay)
		if r.refCounter != 0 {
			t.Errorf("wrong .refCounter value (%d/0)", r.refCounter)
		}
		if !closerExecuted {
			t.Errorf("closer func hasn't been executed\nrefCounter: %d", r.refCounter)
		}
	}) {
		return
	}

	// check listening
	time.Sleep(SleepDelay)
	if v := r.isListening.Get(); v {
		t.Errorf("wrong .isListening value (%v)", v)
	}
}
