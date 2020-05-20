package xlog

import (
	"errors"
	"fmt"
	"runtime"
)

// The error returns when recorder id can not be found in the
// recorder list or if the new id already used in the logger object.
var ErrWrongRecorderID = errors.New("wrong recorder id")

var ErrNotInitialised = errors.New("not initialised")

// TODO: description
var ErrWrongFlagValue = errors.New("wrong flag value")

// The error returns when user tries to write to the empty logger.
var ErrNoRecorders = errors.New("the logger has no registered recorders")

// The error returns when a user tries to write to the logger without
// default recorders with unspecified custom recorders field (nil).
var ErrNotWhereToWrite = errors.New("the logger has no default recorders, " +
	"but custom recorders are not specified")

// The error transmits by recorder listener when it receives unknown signal.
var ErrUnknownSignal = errors.New("unknown signal")

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

type ieType int

const (
	ieCritical ieType = 1 << iota
	ieUnreachable
)

func (e ieType) String() string {
	switch e {
	case ieCritical:
		return "critical"
	case ieUnreachable:
		return "unreachable"
	default:
		return "unknown"
	}
}

type InternalError struct {
	Err  error
	Type ieType
	File string
	Func string
	Line int
}

func (e InternalError) Error() string {
	msg := fmt.Sprintf("[%s] %s internal error: %s",
		e.Func, e.Type.String(), e.Err.Error())
	//msg += fmt.Sprintf("\n(%s:%d)", e.File, e.Line)
	return msg
}

func internalError(t ieType, msgFmt string, msgArgs ...interface{}) error {
	err := InternalError{Type: t}
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
