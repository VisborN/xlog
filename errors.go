package xlog

import (
	"fmt"
	"errors"
)

var NoRecordersError = errors.New("the logger has no registered recorders")

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
