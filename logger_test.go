package xlog

import (
	"container/list"
	"os"
	"runtime"
	"testing"

	"github.com/rs/xid"
)

const showAdditionalInfo = false

const (
	//emsgError             = " return error\n%v"
	emsgErrExpected       = " return nil, error expected"
	emsgUnexpectedError   = "unexpected error\n%v"
	emsgUnexpectedErrType = "unexpected error type\n%v"
	emsgPanicExpected     = "no panic situation"

	lmsgErrOK = "[OK] error > %s"
)

func TestNotListenFreeze(t *testing.T) {
	t.Log("TODO")
	t.SkipNow()
}

func TestLoggerRegistering(t *testing.T) {
	l := NewLogger()
	r := NewIoDirectRecorder(os.Stdout)
	var recID RecorderID = "rec"

	t.Run("RegisterRecorder@OK", func(t *testing.T) {
		if e := l.RegisterRecorder(recID, r.Intrf()); e != nil {
			t.Fatalf("RegisterRecorder() return error\n%s", e.Error())
		}

		//l.RLock()
		//defer l.RUnlock()

		if intrf, exist := l.recorders[recID]; exist {
			if intrf != r.Intrf() {
				t.Error("interface stored in .recorders is wrong")
			}
		} else {
			t.Error("[CRIT] .recorders record doesn't exist")
		}

		if state, exist := l.recordersInit[recID]; exist {
			if state != false {
				t.Error("wrong recorder state (true)")
			}
		} else {
			t.Errorf("[CRIT] .recordersInit record doesn't exist")
		}

		if sevMask, exist := l.severityMasks[recID]; exist {
			if sevMask != SeverityAll {
				t.Error("default severity mask is wrong")
			}
		} else {
			t.Error("[CRIT] .severityMasks record doesn't exist")
		}

		for _, v := range l.defaults {
			if v == recID {
				goto ok
			}
		}
		t.Error("can't find ID in .defaults")
	ok:

		if l.initialised != false {
			t.Error("wrong .initialised state (true)")
		}
	})

	t.Run("RegisterRecorder@WrongID", func(t *testing.T) {
		// secondary call with the same recorder ID
		e := l.RegisterRecorder(recID, r.Intrf())
		if e == nil {
			t.Error("RegisterRecorder()" + emsgErrExpected)
		} else {
			if e != ErrWrongRecorderID {
				t.Errorf(emsgUnexpectedError, e)
			}
		}
	})

	t.Run("RegisterRecorder@Empty", func(t *testing.T) {
		e := l.RegisterRecorder(RecorderID(""), r.Intrf())
		/* PROTECTED
		t.Cleanup(func() {l.UnregisterRecorder(RecorderID(""))})
		*/

		if showAdditionalInfo {
			// outp just to make sure that it's not recorded
			t.Logf("err: %s", e.Error())
			t.Logf("obj: %v", l)
		}

		if e == nil {
			t.Error("RegisterRecorder()" + emsgErrExpected)
		} else {
			if e != ErrWrongParameter {
				t.Errorf(emsgUnexpectedError, e)
			}
		}
	})

	t.Run("RegisterRecorder@NilIntrf", func(t *testing.T) {
		if e := l.RegisterRecorder(RecorderID("some-id"), RecorderInterface{}); e == nil {
			t.Error("RegisterRecorder()" + emsgErrExpected)
		} else if e != ErrWrongParameter {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Run("UnregisterRecorder@WrongID", func(t *testing.T) {
		if e := l.UnregisterRecorder(RecorderID("wrong-rec")); e == nil {
			t.Error("RegisterRecorder()" + emsgErrExpected)
		} else if e != ErrWrongRecorderID {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Run("UnregisterRecorder@DeadLock1", func(t *testing.T) {
		t.Log("LVPRM - SKIP")
		t.SkipNow()
	})

	t.Run("UnregisterRecorder@OK", func(t *testing.T) {
		if e := l.UnregisterRecorder(recID); e != nil {
			t.Fatalf("UnregisterRecorder() return error\n%s", e.Error())
		}

		//l.RLock()
		//defer l.RUnlock()

		for _, v := range l.defaults {
			if v == recID {
				t.Error("found ID in .defaults")
				break
			}
		}

		if l.initialised != false { // we had only one recorder
			t.Error("wrong .initialised value (true) <1 rec. case>")
		}

		if _, exist := l.recorders[recID]; exist {
			t.Error("record in .recorders exists")
		}
		if _, exist := l.recordersInit[recID]; exist {
			t.Error("record in .recordersInit exists")
		}
		if _, exist := l.severityMasks[recID]; exist {
			t.Error("record in .severityMasks exists")
		}
		if _, exist := l.severityOrder[recID]; exist {
			t.Error("record in .severityOrder exists")
		}
	})

	if len(l.recorders) != 0 {
		// just to be sure, prev tests may be failed
		l.recorders = make(map[RecorderID]RecorderInterface)
	}

	t.Run("UnregisterRecorder@NoRec", func(t *testing.T) {
		if e := l.UnregisterRecorder(recID); e == nil {
			t.Error("RegisterRecorder()" + emsgErrExpected)
		} else if e != ErrNoRecorders {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Run("UnregisterRecorder@Empty", func(t *testing.T) {
		if e := l.UnregisterRecorder(RecorderID("")); e == nil {
			t.Error("UnregisterRecorder()" + emsgErrExpected)
		} else if e != ErrWrongParameter {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	var l0 Logger
	// create condition for the error/panic
	l0.recorders = make(map[RecorderID]RecorderInterface)
	l0.recorders[recID] = /* r.Intrf() */ RecorderInterface{}

	t.Run("UnregisterRecorder@internal", func(t *testing.T) {
		if e := l0.UnregisterRecorder(recID); e == nil {
			t.Error("UnregisterRecorder()" + emsgErrExpected)
		} else {
			if _, ok := e.(InternalError); !ok {
				t.Errorf(emsgUnexpectedError, e)
			}
		}
	})

	t.Run("UnregisterRecorder@panic", func(t *testing.T) {
		t.SkipNow() // <--- SKIP
		l0.recordersInit = make(map[RecorderID]bool)
		t.Log("should panics")
		_ = l0.UnregisterRecorder(recID)
		t.Error(emsgPanicExpected)
	})

	t.Run("UnregisterRecorder@panic2", func(t *testing.T) { // nil channel
		t.SkipNow() // <--- SKIP
		l0.recordersInit = make(map[RecorderID]bool)
		l0.recordersInit[recID] = true
		t.Log("should panics")
		_ = l0.UnregisterRecorder(recID)
		t.Error(emsgPanicExpected)
	})
}

// UNSAFE, use in tests only
func (rid RecorderID) verify(src []RecorderID) RecorderID {
	for _, v := range src {
		if v == rid {
			return rid
		}
	}
	panic("recorder id validation")
}

func TestLoggerInitialisation(t *testing.T) {

	// recorder state checks -> rec. tests //

	l := NewLogger()
	r1 := NewIoDirectRecorder(os.Stdout)
	r2 := NewIoDirectRecorder(os.Stdout)
	var rec1ID RecorderID = "rec-1"
	var rec2ID RecorderID = "rec-2"

	go r1.Listen()
	go r2.Listen()
	defer func() { r1.Intrf().ChCtl <- SignalStop() }()
	defer func() { r2.Intrf().ChCtl <- SignalStop() }()

	// don't want to print logs everywhere, recorders just should listen
	t.Log("CAREFUL, Initialise() calls may freeze the program")

	t.Run("Initialise@NoRec", func(t *testing.T) {
		if e := l.Initialise(); e == nil {
			t.Error("Initialise()" + emsgErrExpected)
		} else if e != ErrNoRecorders {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Run("Initialise@partial", func(t *testing.T) {
		// an initialisation error in one of the recorders
		if e := l.RegisterRecorder(rec1ID, r1.Intrf()); e != nil {
			t.Fatalf("RegisterRecorder() return error\n%s", e.Error())
		}
		t.Cleanup(func() { l.UnregisterRecorder(rec1ID) })
		if e := l.RegisterRecorder(rec2ID, r2.Intrf()); e != nil {
			t.Fatalf("RegisterRecorder() return error\n%s", e.Error())
		}
		t.Cleanup(func() { l.UnregisterRecorder(rec2ID) })

		l._falseInit.add(rec2ID.verify(l.defaults))
		t.Cleanup(func() { l._falseInit = nil })

		if e := l.Initialise(); e == nil {
			t.Error("Initialise()" + emsgErrExpected)
		} else {
			if br, ok := e.(BatchResult); ok {
				if failed := br.GetSuccessful(); len(failed) != 1 || failed[0] != rec1ID {
					t.Errorf("unexpected BatchResult.successful value\n%v", br.successful)
				}
				if _, exist := br.GetErrors()[rec2ID]; !exist || len(br.errors) != 1 {
					t.Errorf("unexpected BatchResult.errors value\n%v", br.errors)
				} else if showAdditionalInfo {
					//t.Logf("[OK] BatchResult message: %s", br.Error())
					t.Logf(lmsgErrOK, br.Error())
				}
				// check Logger's fields
				if v := l.recordersInit[rec1ID]; v != true {
					t.Errorf("wrong .recordersInit[%s] value (%v)", rec1ID, v)
				}
				if v := l.recordersInit[rec2ID]; v != false {
					t.Errorf("wrong .recordersInit[%s] value (%v)", rec2ID, v)
				}
			} else {
				t.Errorf(emsgUnexpectedError, e)
			}
		}
	})

	t.Run("Initialise@OK", func(t *testing.T) {
		if e := l.RegisterRecorder(rec1ID, r1.Intrf()); e != nil {
			t.Fatalf("RegisterRecorder() return error\n%s", e.Error())
		}
		if e := l.RegisterRecorder(rec2ID, r2.Intrf()); e != nil {
			t.Fatalf("RegisterRecorder() return error\n%s", e.Error())
		}
		if e := l.Initialise(); e != nil {
			t.Errorf("Initialise() return error\n%s", e.Error())
		} else {
			if v := l.recordersInit[rec1ID]; v != true {
				t.Errorf("wrong .recordersInit[%s] value (%v)", rec1ID, v)
			}
			if v := l.recordersInit[rec2ID]; v != true {
				t.Errorf("wrong .recordersInit[%s] value (%v)", rec2ID, v)
			}
			if l.initialised != true {
				t.Error("wrong .initialised value")
			}
		}
	})

	type loggerIVAL struct {
		recordersInit map[RecorderID]bool
		initialised   bool
	}

	t.Run("Initialise@Second", func(t *testing.T) {
		snapshot := loggerIVAL{l.recordersInit, l.initialised}
		if e := l.Initialise(); e != nil {
			t.Fatalf("Initialise()" + emsgErrExpected)
		}

		for rec, val := range snapshot.recordersInit {
			if cur, exist := l.recordersInit[rec]; exist {
				if val != cur {
					t.Errorf(".recordersInit[%s] changed", rec)
				}
			} else { // !exist
				t.Error(".recordersInit missed value")
			}
		}
		if snapshot.initialised != l.initialised {
			t.Errorf(".initialised changed, unexpected value (%v)", l.initialised)
		}
	})

	var l0 Logger
	// just create condition for the error/panic
	l0.recorders = make(map[RecorderID]RecorderInterface)
	l0.recorders[rec1ID] = r1.Intrf()

	t.Run("Initialise@internal", func(t *testing.T) {
		l0.initialised = false

		if e := l0.Initialise(); e == nil {
			t.Error("Initialise()" + emsgErrExpected)
		} else {
			if _, ok := e.(InternalError); !ok {
				t.Errorf(emsgUnexpectedError, e)
			} else if showAdditionalInfo {
				t.Logf(lmsgErrOK, e.Error())
			}
		}
	})

	t.Run("Initialise@panic", func(t *testing.T) {
		t.SkipNow() // <--- SKIP
		l0.severityMasks = make(map[RecorderID]MsgFlagT)
		l0.severityOrder = make(map[RecorderID]*list.List)
		t.Log("should panics")
		_ = l0.Initialise()
		t.Error(emsgPanicExpected)
	})

	t.Run("Close@OK", func(t *testing.T) {
		dc <- DbgMsg(xid.NilID(), "----- TestLoggerInitialisation@Close@OK")
		r1.chDbg = dc
		l.Close()
		runtime.Gosched()
		if v := l.initialised; v {
			t.Errorf("wrong .initialised value (%v)", v)
		}
	})
}

func TestLoggerAutoStartListening(t *testing.T) {
	l := NewLogger()
	r1 := NewIoDirectRecorder(os.Stdout)
	r2 := NewIoDirectRecorder(os.Stdout)
	var rec1ID RecorderID = "rec-1"
	var rec2ID RecorderID = "rec-2"
	go r1.Listen() // only one
	defer func() { r1.Intrf().ChCtl <- SignalStop() }()

	var rlist ListOfRecorders
	rlist.Add(r1)
	rlist.Add(r2)

	if e := l.RegisterRecorder(rec1ID, r1.Intrf()); e != nil {
		t.Fatalf("RegisterRecorder() return error\n%s", e.Error())
	}
	if e := l.RegisterRecorder(rec2ID, r2.Intrf()); e != nil {
		t.Fatalf("RegisterRecorder() return error\n%s", e.Error())
	}

	t.Run("AutoStartListening=OFF", func(tt *testing.T) {
		CfgAutoStartListening.Set(false)
		defer func() { l.recordersInit[rec1ID] = false }()
		if e := l.Initialise(rlist); e == nil {
			tt.Error("Initialise()" + emsgErrExpected)
		} else {
			if _, ok := e.(BatchResult); !ok {
				tt.Errorf(emsgUnexpectedError, e)
			} else if showAdditionalInfo {
				tt.Logf(lmsgErrOK, e.Error())
			}
		}
	})

	t.Run("AutoStartListening=ON", func(tt *testing.T) {
		CfgAutoStartListening.Set(true)
		if e := l.Initialise(rlist); e != nil {
			tt.Errorf("Initialise() return error\n%s", e.Error())
		} else {
			t.Cleanup(func() { r2.Intrf().ChCtl <- SignalStop() })
		}
		if !l.initialised {
			tt.Errorf(".initialised wrong value (%v)", l.initialised)
		}
	})
}

func TestDefaults(t *testing.T) {
	l := NewLogger()
	var rec1ID RecorderID = "rec-1"
	var rec2ID RecorderID = "rec-1"
	//var intrf RecorderInterface

	t.Run("DefaultsSet@NoRec", func(t *testing.T) {
		if e := l.DefaultsSet([]RecorderID{rec1ID}); e == nil {
			t.Error("DefaultsSet()" + emsgErrExpected)
		} else if e != ErrNoRecorders {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	l.recorders[rec1ID] = RecorderInterface{}

	t.Run("DefaultsSet@WrongID", func(t *testing.T) {
		if e := l.DefaultsSet([]RecorderID{rec1ID, "worng-id"}); e == nil {
			t.Error("DefaultsSet()" + emsgErrExpected)
		} else {
			if _, ok := e.(BatchResult); !ok {
				t.Errorf(emsgUnexpectedError, e)
			} else if showAdditionalInfo {
				t.Logf(lmsgErrOK, e.Error())
			}
		}
	})

	t.Run("DefaultsSet@OK", func(t *testing.T) {
		isEqual := func(op1 []RecorderID, op2 []RecorderID) bool {
			if len(op1) != len(op2) {
				return false
			}
		main_iter:
			for _, v1 := range op1 {
				for _, v2 := range op2 {
					if v2 == v1 {
						continue main_iter
					}
				}
				return false // el not found
			}

			return true
		}

		l.defaults = nil                          // internal
		l.recorders[rec2ID] = RecorderInterface{} //

		if e := l.DefaultsSet([]RecorderID{rec1ID, rec2ID}); e != nil {
			t.Errorf("DefaultsSet() return error\n%s", e.Error())
		} else {
			if !isEqual(l.defaults, []RecorderID{rec1ID, rec2ID}) {
				t.Errorf("wrong data value\n.defaults: %v", l.defaults)
			}
		}
	})

	// skip for now, these functions should be the same
	t.Run("DefaultsAdd", func(t *testing.T) { t.SkipNow() })
	t.Run("DefaultsRemove", func(t *testing.T) { t.SkipNow() })
}

func TestSeverities(t *testing.T) {
	l := NewLogger()
	var recID RecorderID = "rec"

	t.Run("ChangeSeverityOrder@NoRec", func(t *testing.T) {
		if e := l.ChangeSeverityOrder(recID, 0, false, 0); e == nil {
			t.Error("ChangeSeverityOrder()" + emsgErrExpected)
		} else if e != ErrNoRecorders {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Run("ChangeSeverityOrder@Empty", func(t *testing.T) {
		if e := l.ChangeSeverityOrder(RecorderID(""), 0, false, 0); e == nil {
			t.Error("ChangeSeverityOrder()" + emsgErrExpected)
		} else if e != ErrWrongParameter {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	if e := l.RegisterRecorder(recID, emptyRecorder); e != nil {
		t.Fatalf("RegisterRecorder() return error\n%s", e.Error())
	}

	t.Run("ChangeSeverityOrder@WrongID", func(t *testing.T) {
		if e := l.ChangeSeverityOrder(RecorderID("wrong-rec"), 0, false, 0); e == nil {
			t.Error("ChangeSeverityOrder()" + emsgErrExpected)
		} else if e != ErrWrongRecorderID {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Run("ChangeSeverityOrder@WrongFlag", func(t *testing.T) {
		t.Log("this functionality is currently disabled (see code)")
		t.SkipNow()
	})

	t.Run("ChangeSeverityOrder@SameInput", func(t *testing.T) {
		if e := l.ChangeSeverityOrder(recID, Info, Before, Info); e == nil {
			t.Error("ChangeSeverityOrder()" + emsgErrExpected)
			t.Logf(".severityOrder: %v", l.severityOrder[recID])
		} else if e != ErrWrongFlagValue {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Run("ChangeSeverityOrder@OK", func(t *testing.T) {
		if e := l.ChangeSeverityOrder(recID, Info, Before, Notice); e != nil {
			t.Fatalf("ChangeSeverityOrder()" + emsgErrExpected)
		}
		expectedSevOrder := list.New().Init()
		expectedSevOrder.PushBack(Emerg)
		expectedSevOrder.PushBack(Alert)
		expectedSevOrder.PushBack(Critical)
		expectedSevOrder.PushBack(Error)
		expectedSevOrder.PushBack(Warning)
		expectedSevOrder.PushBack(Info)   // SWAPED
		expectedSevOrder.PushBack(Notice) //
		expectedSevOrder.PushBack(Debug)
		expectedSevOrder.PushBack(CustomB1)
		expectedSevOrder.PushBack(CustomB2)

		if _, exist := l.severityOrder[recID]; !exist {
			t.Fatalf(".severityOrder is nil")
		}

		if l.severityOrder[recID].Len() != expectedSevOrder.Len() {
			t.Fatalf(".severityOrder map has wrong length\n"+
				"logger's map: %v\nexpected map: %v",
				l.severityOrder[recID], expectedSevOrder)
		}

		el1 := expectedSevOrder.Front()
		el2 := l.severityOrder[recID].Front()
		if el1 == nil || el2 == nil {
			t.Fatal("list.Front() return nil")
		}

		for el1 != nil && el2 != nil {
			var ev, rv MsgFlagT
			var ok bool
			if ev, ok = el1.Value.(MsgFlagT); !ok {
				t.Fatalf("expectedSevOrder wrong value type")
			}
			if rv, ok = el2.Value.(MsgFlagT); !ok {
				t.Fatalf(".severityOrder wrong value type")
			}

			if ev != rv {
				t.Errorf("unexpected sev. order value\n%x, %x expected", rv, ev)
			}

			el1 = el1.Next()
			el2 = el2.Next()
		}

	})

	l0 := Logger{}
	l0.recorders = make(map[RecorderID]RecorderInterface)
	l0.recorders[recID] = RecorderInterface{}

	t.Run("ChangeSeverityOrder@internal", func(t *testing.T) {
		if e := l0.ChangeSeverityOrder(recID, 0, false, 0); e == nil {
			t.Error("ChangeSeverityOrder()" + emsgErrExpected)
		} else {
			if _, ok := e.(InternalError); !ok {
				t.Errorf(emsgUnexpectedErrType, e)
			} else if showAdditionalInfo {
				t.Logf(lmsgErrOK, e.Error())
			}
		}
	})

	t.Run("ChangeSeverityOrder@panic1", func(t *testing.T) {
		t.SkipNow() // <--- SKIP
		l0.severityOrder = make(map[RecorderID]*list.List)
		t.Log("should panics")
		if e := l0.ChangeSeverityOrder(recID, Info, Before, Notice); e != nil {
			t.Fatalf("error, panic expected\n%s", e.Error())
		}
		t.Error(emsgPanicExpected)
	})

	// ----- Severity Masks

	l = NewLogger()

	t.Run("SetSeverityMask@Empty", func(t *testing.T) {
		if e := l.SetSeverityMask(RecorderID(""), 1); e == nil {
			t.Error("SetSeverityMask()" + emsgErrExpected)
		} else if e != ErrWrongParameter {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Run("SetSeverityMask@NoRec", func(t *testing.T) {
		l.severityMasks = make(map[RecorderID]MsgFlagT)
		if e := l.SetSeverityMask(recID, 1); e == nil {
			t.Error("SetSeverityMask()" + emsgErrExpected)
		} else if e != ErrNoRecorders {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	if e := l.RegisterRecorder(recID, emptyRecorder); e != nil {
		t.Fatalf("RegisterRecorder() return error\n%s", e.Error())
	}

	t.Run("SetSeverityMask@WrongRec", func(t *testing.T) {
		if e := l.SetSeverityMask(RecorderID("wrong-rec"), 1); e == nil {
			t.Error("SetSeverityMask()" + emsgErrExpected)
		} else if e != ErrWrongRecorderID {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Run("SetSeverityMask@OK", func(t *testing.T) {
		if e := l.SetSeverityMask(recID, 0); e != nil {
			t.Errorf("SetSeverityMask() return error\n%s", e.Error())
		} else {
			if mask, exist := l.severityMasks[recID]; !exist {
				t.Error(".severityMasks | item doesn't exist")
			} else if mask != 0 {
				t.Errorf(".severityMasks | unexpected value (%x)", mask)
			}
		}
	})

	t.Run("SetSeverityMask@internal", func(t *testing.T) {
		if e := l0.SetSeverityMask(recID, 1); e == nil {
			t.Error("SetSeveritymask()" + emsgErrExpected)
		} else {
			if _, ok := e.(InternalError); !ok {
				t.Errorf(emsgUnexpectedErrType, e)
			} else if showAdditionalInfo {
				t.Logf(lmsgErrOK, e.Error())
			}
		}
	})

	t.Run("SetSeverityMask@panic", func(t *testing.T) {
		t.SkipNow() // <--- SKIP
		l0.severityMasks = make(map[RecorderID]MsgFlagT)
		t.Log("sould panics")
		_ = l0.SetSeverityMask(recID, 1)
		t.Error(emsgPanicExpected)
	})

}

func TestWriteFunc(t *testing.T) {
	l := NewLogger()
	r1 := NewIoDirectRecorder(os.Stdout, "REC-1")
	r2 := NewIoDirectRecorder(os.Stdout, "REC-2")
	var rec1ID RecorderID = RecorderID("rec-1")
	var rec2ID RecorderID = RecorderID("rec-2")

	go r1.Listen()
	go r2.Listen()
	defer func() { r1.Intrf().ChCtl <- SignalStop() }()
	defer func() { r2.Intrf().ChCtl <- SignalStop() }()
	runtime.Gosched()

	// don't want to print logs everywhere, recorders just should listen
	t.Log("CAREFUL, WriteMsg() calls may freeze the program")

	t.Run("WriteMsg@NoRec", func(t *testing.T) {
		l.initialised = true
		t.Cleanup(func() {
			l.initialised = false
		})

		if e := l.WriteMsg([]RecorderID{rec1ID}, NewLogMsg()); e == nil {
			t.Error("WriteMsg()" + emsgErrExpected)
		} else if e != ErrNoRecorders {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	if e := l.RegisterRecorder(rec1ID, r1.Intrf(), false); e != nil {
		t.Fatalf("RegisterRecorder() return error (%s)\n%v", rec1ID, e)
	}
	if e := l.RegisterRecorder(rec2ID, r2.Intrf(), false); e != nil {
		t.Fatalf("RegisterRecorder() return error (%s)\n%v", rec2ID, e)
	}

	t.Run("WriteMsg@NotInit", func(t *testing.T) {
		if e := l.WriteMsg([]RecorderID{rec1ID}, NewLogMsg()); e == nil {
			t.Error("WriteMsg() return nil, error expected")
			t.Error("WriteMsg()" + emsgErrExpected)
		} else if e != ErrNotInitialised {
			t.Errorf(emsgUnexpectedError, e)
		}
	})

	t.Log("initialising the logger...")
	if e := l.Initialise(); e != nil {
		t.Fatalf("Initialise() return error\n%s", e.Error())
	}
	t.Log("done")

	t.Run("WriteMsg@NoWriteTrg", func(t *testing.T) {
		if e := l.WriteMsg(nil, NewLogMsg()); e == nil {
			t.Error("WriteMsg()" + emsgErrExpected)
		} else if e != ErrNotWhereToWrite {
			t.Errorf(emsgUnexpectedError, e)
		} else if showAdditionalInfo {
			t.Log("[OK] nil case")
		}
		if e := l.WriteMsg([]RecorderID{}, NewLogMsg()); e == nil {
			t.Error("WriteMsg()" + emsgErrExpected)
		} else if e != ErrNotWhereToWrite {
			t.Errorf(emsgUnexpectedError, e)
		} else if showAdditionalInfo {
			t.Log("[OK] empty array case")
		}
	})

	if e := l.DefaultsSet([]RecorderID{rec1ID}); e != nil {
		t.Fatalf("DefaultsSet() return error\n%v", e)
	}

	t.Run("WriteMsg@partial@WrongRec", func(t *testing.T) {
		defer runtime.Gosched() // GOSCHED
		if e := l.WriteMsg([]RecorderID{rec1ID, "wrong-rec"}, NewLogMsg()); e == nil {
			t.Error("WriteMsg()" + emsgErrExpected)
		} else {
			if err, ok := e.(BatchResult); !ok {
				t.Errorf(emsgUnexpectedErrType, e)
			} else if showAdditionalInfo {
				t.Logf(lmsgErrOK, err)
			}
		}
	})

	t.Run("WriteMsg@NilMsg", func(t *testing.T) {

		// --------------------
		t.Log("SKIP UNTIL FIX")
		t.SkipNow()
		// --------------------

		if e := l.WriteMsg(nil, nil); e == nil {
			t.Error("WriteMsg()" + emsgErrExpected)
		} else if e != ErrWrongParameter {
			t.Error(emsgUnexpectedError, e)
		}
	})

	t.Run("WriteMsg@partial@SevPrtErr", func(t *testing.T) { t.SkipNow() })

	t.Run("WriteMsg@OK@nil", func(t *testing.T) {
		if e := l.WriteMsg(nil, NewLogMsg().Setf("@OK@nil")); e != nil {
			t.Errorf("WriteMsg() return error\n%s", e.Error())
		} else {
			t.Log("you should see the output only from REC-1")
			runtime.Gosched() // GOSCHED
		}
	})

	t.Run("WriteMsg@OK@custom", func(t *testing.T) {
		if e := l.WriteMsg([]RecorderID{rec1ID, rec2ID}, NewLogMsg().Setf("@OK@custom")); e != nil {
			t.Errorf("WriteMsg() return error\n%s", e.Error())
		} else {
			t.Log("you should see the output from BOTH recorders")
			runtime.Gosched() // GOSCHED
		}
	})

	l0 := Logger{}
	l0.initialised = true
	l0.recorders = make(map[RecorderID]RecorderInterface)
	l0.recorders[RecorderID("some-rec")] = RecorderInterface{}
	l0.defaults = append(l0.defaults, RecorderID("some-rec"))

	t.Run("WriteMsg@internal", func(t *testing.T) {
		if e := l0.WriteMsg([]RecorderID{}, NewLogMsg()); e == nil {
			t.Error("WriteMsg()" + emsgErrExpected)
		} else {
			if _, ok := e.(InternalError); !ok {
				t.Errorf(emsgUnexpectedErrType, e)
			} else if showAdditionalInfo {
				t.Logf(lmsgErrOK, e.Error())
			}
		}
	})

	l0.severityOrder = make(map[RecorderID]*list.List)
	l0.severityOrder["some-rec"] = defaultSeverityOrder()
	l0.severityMasks = make(map[RecorderID]MsgFlagT)
	//l0.severityMasks["some-rec"] = SeverityAll

	t.Run("WriteMsg@panic", func(t *testing.T) {
		t.SkipNow() // <--- SKIP
		t.Log("should panics")
		_ = l0.WriteMsg([]RecorderID{"some-rec"}, NewLogMsg())
		t.Error(emsgPanicExpected)
	})
}
