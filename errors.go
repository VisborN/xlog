package xlog

import (
	"fmt"
	"errors"
)

var NoRecordersError = errors.New("the logger has no registered recorders")
var NotInitialised = errors.New("can't write, the recorder is not initialised")

// -----------------------------------------------------------------------------

type RecordersError struct { // multiple recorders error
	Recorders []RecorderID
	err error
}

func (e RecordersError) Error() string {
	if len(e.Recorders) == 0 {
		return e.err.Error()
	}
	var msg string
	if e.err.Error() != "" {
		msg = e.err.Error() + " ("
	} else {
		msg = "<unknown> ("
	}
	for _, recID := range e.Recorders {
		msg += fmt.Sprintf("%s, ", recID)
	}
	msg = msg[:len(msg)-2] + ")"
	return msg
}

func (e RecordersError) Add(recorder RecorderID) RecordersError {
	e.Recorders = append(e.Recorders, recorder); return e
}

func (e RecordersError) NotEmpty() bool {
	if len(e.Recorders) > 0 { return true }
	return false
}

// -----------------------------------------------------------------------------

type InitialisationError struct {
	RecordersErrors map[RecorderID]error
	ErrorsInAll bool
}

func errInitialisationError() InitialisationError {
	e := InitialisationError{}
	e.RecordersErrors =
		make(map[RecorderID]error)
	return e
}

func (e InitialisationError) Error() string {
	msg := "errors occurred during initialising recorders:"
	for recID, _ := range e.RecordersErrors {
		msg += fmt.Sprintf(" %s,", recID)
	}
	return msg[:len(msg)-1]
}

func (e InitialisationError) SetAll() InitialisationError {
	e.ErrorsInAll = true; return e
}
