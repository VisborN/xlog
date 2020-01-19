package xlog

import (
	"fmt"
	"time"
	"errors"
)

const ( // severity flags (log level)
	Critical uint16 = 0x01 // 0000 0000 0000 0001
	Error    uint16 = 0x02 // 0000 0000 0000 0010
	Warning  uint16 = 0x04 // 0000 0000 0000 0100
	Notice   uint16 = 0x08 // 0000 0000 0000 1000
	Info     uint16 = 0x10 // 0000 0000 0001 0000
	Debug1   uint16 = 0x20 // 0000 0000 0010 0000
	Debug2   uint16 = 0x40 // 0000 0000 0100 0000
	Debug3   uint16 = 0x80 // 0000 0000 1000 0000
)

const SeverityAll   uint16 = 0xFF
const SeverityMajor uint16 = 0x0F
const SeverityMinor uint16 = 0xF0

type logMsg struct {
	time time.Time
	severity uint16
	content string
	data interface{}
}

// NewLogMsg allocates and returns a new logMsg. 
func NewLogMsg() *logMsg {
	lm := new(logMsg)
	lm.time = time.Now()
	return lm
}

// LogMsg builds and returns simple message with undefined severity.
func LogMsg(msgFmt string, msgArgs ...interface{}) *logMsg {
	lm := new(logMsg)
	lm.time = time.Now()
	lm.content = fmt.Sprintf(msgFmt, msgArgs...)
	return lm
}

// Severity sets severity value for the message.
func (LM *logMsg) Severity(severity uint16) *logMsg {
	LM.severity = severityProtector(severity); return LM
}

// UpdateTime updates message's time to current time.
func (LM *logMsg) UpdateTime() *logMsg {
	LM.time = time.Now(); return LM
}

// Add attaches new string to the end of the existing messages text.
func (LM *logMsg) Add(msgFmt string, msgArgs ...interface{}) *logMsg {
	LM.content += fmt.Sprintf(msgFmt, msgArgs...); return LM
}

// AddLn adds new string to existing message text as a new line.
func (LM *logMsg) AddLn(msgFmt string, msgArgs ...interface{}) *logMsg {
	LM.content += "\n" + fmt.Sprintf(msgFmt, msgArgs...); return LM
}

// Set resets current message's text and sets the given string.
func (LM *logMsg) Set(msgFmt string, msgArgs ...interface{}) *logMsg {
	LM.content = fmt.Sprintf(msgFmt, msgArgs...); return LM
}

// -----------------------------------------------------------------------------

type FormatFunc func(logMsg) string

type logRecorder interface {
	initialise() error
	//isInitialised() bool
	close()
	write(logMsg)
}

type RecorderID string

type Logger struct {
	recorders map[RecorderID]logRecorder
	sevMapping map[RecorderID]uint16
	defaults []RecorderID // list of default recorders
}

// The same as RegisterRecorderEx, but adds recorder to defaults automatically.
func (L *Logger) RegisterRecorder(id RecorderID, recorder logRecorder) bool {
	return L.RegisterRecorderEx(id, true, recorder)
}

// RegisterRecorder registers the recorder in the logger with the given id.
// An asDefault parameter says whether the need to set it as default recorder.
//
// The function returns true on success, and false if the given id is already bound.
//
// This function can panic if it found a critical error in the logger data.
func (L *Logger) RegisterRecorderEx(id RecorderID, asDefault bool, recorder logRecorder) bool {
	if L.recorders == nil {
		L.recorders = make(map[RecorderID]logRecorder)
	} else {
		for recID, _ := range L.recorders {
			if recID == id { return false }
		}
	}
	L.recorders[id] = recorder

	// recorder works on all severities by default
	if L.sevMapping == nil {
		L.sevMapping = make(map[RecorderID]uint16)
	}
	L.sevMapping[id] = SeverityAll

	// check for ensure that it's correct
	for _, recID := range L.defaults {
		if recID == id {
			panic("xlog: impossible identifier found in default recorders list")
		}
	}

	// set as default if necessary
	if asDefault {
		L.defaults = append(L.defaults, id)
	}

	return true
}

// Initialise calls initialisation functions of each registered recorder.
//
// If all of them return nil (no error), this function also returns nil.
// If some initialisation functions return error, this function returns
// InitialisationError with map of failed recorders and them errors.
//
// Be careful, even if it returns an error, some of the recorders can be
// initialised successfully. You can get a global processing status by
// InitialisationError.ErrorInAll. If it has false value, it means that
// some of the recorders have been initialised.
func (L *Logger) Initialise() error {
	if L.recorders == nil {
		return NoRecordersError
	}

	e := errInitialisationError()
	for id, rec := range L.recorders {
		if err := rec.initialise(); err != nil {
			e.RecordersErrors[id] = err
		}
	}
	if l := len(e.RecordersErrors); l > 0 {
		if l == len(L.recorders) { e.SetAll() }
		return e
	}

	return nil
}

// Close calls closing functions of each registered recorder.
func (L *Logger) Close() {
	if L.recorders == nil { return }
	for _, rec := range L.recorders {
		rec.close()
	}
}

// AddToDefaults adds given recorders to defaults in that logger.
//
// (The default recorders list determinate which recorders will use for
// writing if custom recorders are not specified in the log message.)
func (L *Logger) AddToDefaults(recorders []RecorderID) error {
	if L.recorders == nil { return NoRecordersError }

	// check registered recorders
	notRegisteredErr := RecordersError{
		err: errors.New("some of the given recorders are not registered"),
	}
	for _, recID := range recorders {
		if _, exist := L.recorders[recID]; !exist {
			notRegisteredErr.Add(recID)
		}
	}
	if notRegisteredErr.NotEmpty() {
		return notRegisteredErr
	}

main_iter:
	for _, recID := range recorders {

		// check defaults for duplicate
		for _, defID := range L.defaults {
			if defID == recID { // id already in the defaults
				continue main_iter; // skip this recorder
			}
		}

		L.defaults = append(L.defaults, recID)
	}

	return nil
}

// RemoveFromDefaults removes given recorders form defaults in that logger.
//
// (The default recorders list determinate which recorders will use for
// writing if custom recorders are not specified in the log message.)
func (L *Logger) RemoveFromDefaults(recorders []RecorderID) error {
	if L.recorders == nil { return NoRecordersError }

	// check registered recorders
	notRegisteredErr := RecordersError{
		err: errors.New("some of the given recorders are not registered"),
	}
	for _, recID := range recorders {
		if _, exist := L.recorders[recID]; !exist {
			notRegisteredErr.Add(recID)
		}
	}
	if notRegisteredErr.NotEmpty() {
		return notRegisteredErr
	}

	// delete given ids from defaults
	for _, recID := range recorders {
		for i, defID := range L.defaults {
			if defID == recID {
				L.defaults[i] = L.defaults[len(L.defaults)-1]
				//L.defaults[len(L.defaults)-1] = RecorderID("")
				L.defaults = L.defaults[:len(L.defaults)-1]
			}
		}
	}

	return nil
}

// Write builds the message with format line and specified severity flag, then calls
// WriteMsg. It allows avoiding calling fmt.Sprintf() function and logMsg's functions
// directly, it wraps them. Returns nil in case of success otherwise returns an error.
func (L *Logger) Write(severity uint16, msgFmt string, msgArgs ...interface{}) error {
	msg := NewLogMsg().Severity(severity)
	msg.Set(msgFmt, msgArgs...)
	return L.WriteMsg(nil, msg)
}

// WriteMsg writes given message using the specified recorders of this logger.
// If custom recorders are not specified, uses default recorders. Returns nil
// on success and error on fail.
//
// This function can invoke panic in case of critical errors (usually unreachable).
//
// TODO: additional log/outp notifications at errors
func (L *Logger) WriteMsg(recorders []RecorderID, msg *logMsg) error {
	if L.recorders == nil { return NoRecordersError }
	if len(recorders) == 0 && len(L.defaults) == 0 {
		return errors.New(
			"the logger has no default recorders, "+
			"but custom recorders are not specified")
	}

	if L.sevMapping == nil {
		panic("xlog: bumped to nil (severity mapping is not initialised)")
	}

	if len(recorders) > 0 {
		// check registered recorders
		notRegisteredErr := RecordersError{
			err: errors.New("some of the given recorders are not registered"),
		}
		for _, recID := range recorders {
			if _, exist := L.recorders[recID]; !exist {
				notRegisteredErr.Add(recID)
			}
		}
		if notRegisteredErr.NotEmpty() {
			return notRegisteredErr
		}
	} else { // use default recorders
		recorders = L.defaults
	}

	for _, recID := range recorders {
		if sev, exist := L.sevMapping[recID]; exist {
			if sev == 0 { panic("xlog: severity mask is zero") }
			if (*msg).severity == 0 { panic("xlog: msg.sev = 0") } // TODO: remove, not here
			if sev & (*msg).severity > 0 {
				if rec, exist := L.recorders[recID]; exist {
					rec.write(*msg) // <---
				} else {
					panic("xlog: recorder id can't be found in registered recorders map")
				}
			}
		} else {
			panic("xlog: recorder id can't be found in severity mapping")
		}
	}

	return nil
}

// -----------------------------------------------------------------------------

// This function actually has got a protector role because in some places
// a severity argument should have only one of these flags. So it ensures
// (accordingly to the depth order) that severity value provide only one
// flag.
func severityProtector(sev uint16) uint16 {
	if sev & Critical > 0 { return Critical }
	if sev & Error    > 0 { return Error }
	if sev & Warning  > 0 { return Warning }
	if sev & Notice   > 0 { return Notice }
	if sev & Info     > 0 { return Info }
	if sev & Debug1   > 0 { return Debug1 }
	if sev & Debug2   > 0 { return Debug2 }
	if sev & Debug3   > 0 { return Debug3 }
	return 0
}
