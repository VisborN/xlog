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

// ErrWrongFlagValue returns when some function detects a wrong flag value.
var ErrWrongFlagValue = errors.New("xlog: wrong flag value")

// ErrWrongParameter returns when passed parameter is incorrect e.g. RecorderID("").
var ErrWrongParameter = errors.New("xlog: wrong parameter")

// ErrNoRecorders returns when user tries to write to the empty logger.
var ErrNoRecorders = errors.New("xlog: the logger has no registered recorders")

// ErrNotWhereToWrite returns when a user tries to write to the logger without
// configured default recorders with unspecified custom recorders field (nil).
var ErrNotWhereToWrite = errors.New("xlog: " +
	"the logger has no default recorders, " +
	"but custom recorders are not specified")

// ErrNotListening returns when Logger tries to send a signal
// to recorder which is not ready to receive signals.
var ErrNotListening error = errors.New("xlog: recorder is not listening")

/* DEPRECATED
// The error transmits by recorder listener when it receives unknown signal.
var ErrUnknownSignal = errors.New("unknown signal") */

// errMsgBumpedToNil returns in internal error report when Logger
// found uninitialised fields. Try to call NewLogger() first.
var errMsgBumpedToNil = "bumped to nil"

// additional return protection (always unreachable currently)
var errBLOP = errors.New("THIS CODE WAS SUPPOSED TO PANIC")

// it used for tests, shouldn't be exported or documented
var _ErrFalseInit error = errors.New("[OK] false initialisation")

// -----------------------------------------------------------------------------

// BatchResult is designed to accumulate the status (success and failure)
// of several operations. It is used as a partial error when some operations
// complete successfully and some do not. It is mainly used in processing lists
// of recorders, for example during initialisation ops.
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

// This error used for critical situations caused by wrong package usage
// (by user). The operations cannot be done with the wrong data, may cause
// data damage or panics (in this function call).
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

// This function returns a InternalError, see InternalError type
// description for more info. You should call return after this func.
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

/*
// The same as internalError, but receives a predefined error.
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
*/

// This function used for critical and unreachable errors. It should
// cause panic because it represents an issue in xlog code and further
// work is not possible in most cases.
func internalCritical(msg string) error {
	panic(msg)

	return errBLOP // unreachable
}
