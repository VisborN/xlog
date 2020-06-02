package xlog

import (
	"os"
	"testing"
)

var emptyRecorderID RecorderID = ""

func TestLoggerRegistering(t *testing.T) {
	l := NewLogger()
	r := NewIoDirectRecorder(os.Stdout)
	var recID RecorderID = "rec"

	t.Run("RegisterRecorder@OK", func(t *testing.T) {
		e := l.RegisterRecorder(recID, r.Intrf())
		if e != nil {
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
		e := l.RegisterRecorder(recID, r.Intrf())
		if e == nil {
			t.Error("RegisterRecorder() return nil, error expected")
		} else {
			if e != ErrWrongRecorderID {
				t.Errorf("unexpected error\n%v", e)
			}
		}
	})

	t.Run("RegisterRecorder@Empty", func(t *testing.T) {
		e := l.RegisterRecorder(emptyRecorderID, r.Intrf())
		//t.Cleanup(func() {l.UnregisterRecorder(emptyRecorderID)})

		//t.Logf("err: %v", e)
		//t.Logf("obj: %v", l)
		if e == nil {
			t.Error("RegisterRecorder() return nil, error expected")
		} else {
			if e != ErrWrongParameter {
				t.Errorf("unexpected error\n%v", e)
			}
		}
	})

	t.Run("UnregisterRecorder@WrongID", func(t *testing.T) {
		e := l.UnregisterRecorder(RecorderID("wrong-id"))
		if e == nil {
			t.Error("RegisterRecorder() return nil, error expected")
		} else {
			if e != ErrWrongRecorderID {
				t.Errorf("unexpected error\n%v", e)
			}
		}
	})

	t.Run("UnregisterRecorder@DeadLock1", func(t *testing.T) {
		t.SkipNow()
	})

	t.Run("UnregisterRecorder@OK", func(t *testing.T) {
		e := l.UnregisterRecorder(recID)
		if e != nil {
			t.Fatalf("UnregisterRecorder() return error\n%v", e)
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

	t.Run("UnregisterRecorder@NoRec", func(t *testing.T) {
		e := l.UnregisterRecorder(recID)
		if e == nil {
			t.Error("RegisterRecorder() return nil, error expected")
		} else {
			if e != ErrNoRecorders {
				t.Errorf("unexpected error\n%v", e)
			}
		}
	})

	t.Run("UnregisterRecorder@Empty", func(t *testing.T) {
		if e := l.UnregisterRecorder(emptyRecorderID); e == nil {
			t.Error("UnregisterRecorder() return nil, error expected")
		} else {
			if e != ErrWrongParameter {
				t.Errorf("unexpected error\n%v", e)
			}
		}
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

	// recorder state checks -> rec. tests

	l := NewLogger()
	r1 := NewIoDirectRecorder(os.Stdout)
	r2 := NewIoDirectRecorder(os.Stdout)
	var rec1ID RecorderID = "rec-1"
	var rec2ID RecorderID = "rec-2"

	go r1.Listen()
	go r2.Listen()
	defer func() { r1.Intrf().ChCtl <- SignalStop() }()
	defer func() { r2.Intrf().ChCtl <- SignalStop() }()

	t.Run("Initialise@NoRec", func(t *testing.T) {
		e := l.Initialise()
		if e == nil {
			t.Error("Initialise() return nil, error expected")
		} else {
			if e != ErrNoRecorders {
				t.Errorf("unexpected error\n%v", e)
			}
		}
	})

	t.Run("Initialise@Partial", func(t *testing.T) {
		if e := l.RegisterRecorder(rec1ID, r1.Intrf()); e != nil {
			t.Fatalf("RegisterRecorder() return error\n%v", e)
		}
		t.Cleanup(func() { l.UnregisterRecorder(rec1ID) })
		if e := l.RegisterRecorder(rec2ID, r2.Intrf()); e != nil {
			t.Fatalf("RegisterRecorder() return error\n%v", e)
		}
		t.Cleanup(func() { l.UnregisterRecorder(rec2ID) })

		l._falseInit.add(rec2ID.verify(l.defaults))
		t.Cleanup(func() { l._falseInit = nil })

		if e := l.Initialise(); e == nil {
			t.Error("Initialise() return nil, error expected")
		} else {
			if br, ok := e.(BatchResult); ok {
				if failed := br.GetSuccessful(); len(failed) != 1 || failed[0] != rec1ID {
					t.Errorf("unexpected BatchResult.successful value\n%v", br.successful)
				}
				if _, exist := br.GetErrors()[rec2ID]; !exist || len(br.errors) != 1 {
					t.Errorf("unexpected BatchResult.errors value\n%v", br.errors)
				} else {
					t.Logf("[OK] BatchResult message: %s", br.Error())
				}
			} else {
				t.Errorf("unexpected error\n%v", e)
			}
		}
	})

	t.Run("Initialise@OK", func(t *testing.T) {
		if e := l.RegisterRecorder(rec1ID, r1.Intrf()); e != nil {
			t.Fatalf("RegisterRecorder() return error\n%v", e)
		}
		if e := l.RegisterRecorder(rec2ID, r2.Intrf()); e != nil {
			t.Fatalf("RegisterRecorder() return error\n%v", e)
		}
		if e := l.Initialise(); e != nil {
			t.Errorf("Initialise() return error\n%v", e)
		}
	})

	t.Run("Initialise@Second", func(t *testing.T) {
		type loggerIVAL struct {
			recordersInit map[RecorderID]bool
			initialised   bool
		}
		snapshot := loggerIVAL{l.recordersInit, l.initialised}
		if e := l.Initialise(); e != nil {
			t.Fatalf("Initialise() return error\n%v", e)
		}

		for rec, val := range snapshot.recordersInit {
			if cur, exist := l.recordersInit[rec]; exist {
				if val != cur {
					t.Errorf(".recordersInit[%s] had changed", rec)
				}
			} else { // !exist
				t.Error(".recordersInit missed value")
			}
		}
		if snapshot.initialised != l.initialised {
			t.Error("unexpected .initialised value")
		}
	})

	/* TODO (panic_cfg)
	t.Run("Initialise@panic", func(t *testing.T) {
		var l0 Logger

		e := l0.RegisterRecorder(rec1ID, r1.Intrf()) // to prevent ErrNoRecorders case
		if e != nil {
			t.Fatalf("RegisterRecorder() return error\n%v", e)
		}

		// INSERT -> prevent_panic

		e = l0.Initialise()
		if e == nil {
			t.Error("Initialise() return nil")
		} else {

			// TODO: check

		}
	})
	*/

}
