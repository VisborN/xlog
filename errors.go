package xlog

import (
	"errors"
	"fmt"
	"runtime"
)

// ErrWrongRecorderID returns when recorder id can not be found in the
// recorder list or if the new id already used in the logger object.
var ErrWrongRecorderID = errors.New("xlog: wrong recorder id")

// ErrNotInitialised returns when you try to write in the uninitialised logger.
var ErrNotInitialised = errors.New("xlog: not initialised")

// TODO: description
var ErrWrongFlagValue = errors.New("xlog: wrong flag value")

// ErrNoRecorders returns when user tries to write to the empty logger.
var ErrNoRecorders = errors.New("xlog: the logger has no registered recorders")

// ErrNotWhereToWrite returns when a user tries to write to the logger without
// configured default recorders with unspecified custom recorders field (nil).
var ErrNotWhereToWrite = errors.New("xlog: " +
	"the logger has no default recorders, " +
	"but custom recorders are not specified")

/* DEPRECATED
// The error transmits by recorder listener when it receives unknown signal.
var ErrUnknownSignal = errors.New("unknown signal") */

// ErrInternalBumpedToNil returns in internal error report when Logger
// found uninitialised fields. Try to call NewLogger() first.
var errInternalBumpedToNil = "bumped to nil"

// it used for tests, shouldn't be exported or documented
var _ErrFalseInit error = errors.New("[OK] false initialisation")

// -----------------------------------------------------------------------------

// BatchResult using to accumulate statuses (success and failed) of several
// operations and used as kinda a partial error when some operations are
// succeeded and some not. It's used in handling lists of recorders, like
// initialisation ops.
type BatchResult struct {
	errors     map[RecorderID]error
	successful []RecorderID
	errMessage string
}

func (br BatchResult) Error() string {
	if len(br.errors) == 0 {
		return "successful, no errors"
	}

	rlist := " ("
	for rec, _ := range br.errors {
		rlist += fmt.Sprintf("%s, ", rec)
	}
	rlist = rlist[:len(rlist)-2] + ")"
	if br.errMessage == "" {
		br.errMessage = "unknown errors"
	}
	return br.errMessage + rlist
}

func (br BatchResult) GetErrors() map[RecorderID]error {
	if len(br.errors) == 0 {
		// cause by calling .Fail().Ok()
		// we can get a non-nil map
		return nil
	}
	return br.errors
}

func (br BatchResult) GetSuccessful() []RecorderID {
	return br.successful
}

func (br *BatchResult) Fail(rec RecorderID, err error) *BatchResult {
	if br.errors == nil {
		br.errors = make(map[RecorderID]error)
	}
	br.errors[rec] = err

	// check success list
	for i, recID := range br.successful {
		if recID == rec {
			// delete record from success list
			br.successful[i] = br.successful[len(br.successful)-1]
			br.successful[len(br.successful)-1] = ""
			br.successful = br.successful[:len(br.successful)-1]
			break // no duplicates possible
		}
	}

	return br
}

func (br *BatchResult) OK(rec RecorderID) *BatchResult {
	// check for duplicates
	for _, recID := range br.successful {
		if recID == rec {
			return br
		}
	}

	br.successful = append(br.successful, rec)

	// check error list
	for recID, _ := range br.errors {
		if recID == rec {
			delete(br.errors, rec)
			break // no duplicates possible
		}
	}

	return br
}

func (br *BatchResult) SetMsg(msgFmt string, msgArgs ...interface{}) *BatchResult {
	br.errMessage = fmt.Sprintf(msgFmt, msgArgs...)
	return br
}

// -----------------------------------------------------------------------------

type InternalError struct {
	Err  error
	File string
	Func string
	Line int
}

func (e InternalError) Error() string {
	msg := fmt.Sprintf("xlog: [func %s] internal error: %s", e.Func, e.Err.Error())
	//msg += fmt.Sprintf(" (%s:%d)", e.File, e.Line)
	return msg
}

func internalError(msgFmt string, msgArgs ...interface{}) error {
	err := InternalError{}
	pc := make([]uintptr, 20)
	if n := runtime.Callers(2, pc); n != 0 {
		frames := runtime.CallersFrames(pc[:n])
		frame, _ := frames.Next()
		err.File = frame.File
		err.Func = frame.Function
		err.Line = frame.Line
	}
	err.Err = fmt.Errorf(msgFmt, msgArgs...)
	return err
}

func internalErrorPredef(e error) error {
	err := InternalError{}
	pc := make([]uintptr, 20)
	if n := runtime.Callers(2, pc); n != 0 {
		frames := runtime.CallersFrames(pc[:n])
		frame, _ := frames.Next()
		err.File = frame.File
		err.Func = frame.Function
		err.Line = frame.Line
	}
	err.Err = e
	return err
}
