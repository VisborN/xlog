package xlog

import (
	"fmt"
	"time"
	"errors"
	"container/list"
)

/*  xlog message flags

    xxxx xxxx xxxx xxxx
    -+-- --+- ----+----
     |     |      |
attributes |  defeult severity flags
        custom severity flags
*/

const ( // severity flags (log level)
	Critical uint16 = 0x01  // 0000 0000 0000 0001
	Error    uint16 = 0x02  // 0000 0000 0000 0010
	Warning  uint16 = 0x04  // 0000 0000 0000 0100
	Notice   uint16 = 0x08  // 0000 0000 0000 1000
	Info     uint16 = 0x10  // 0000 0000 0001 0000
	Debug1   uint16 = 0x20  // 0000 0000 0010 0000
	Debug2   uint16 = 0x40  // 0000 0000 0100 0000
	Debug3   uint16 = 0x80  // 0000 0000 1000 0000

	Custom1  uint16 = 0x100 // 0000 0001 0000 0000
	Custom2  uint16 = 0x200 // 0000 0010 0000 0000
	Custom3  uint16 = 0x300 // 0000 0100 0000 0000
	Custom4  uint16 = 0x400 // 0000 1000 0000 0000
)

// bit-reset (reversed) mask for severity flags
const severityShadowMask uint16 = 0xF000
// bit-reset (reversed) mask for attribute flags
const attributeShadowMask uint16 = 0x0FFF

// predifined severity sets (utility)
const SeverityAll   uint16 = 0xFF
const SeverityMajor uint16 = 0x0F
const SeverityMinor uint16 = 0xF0
const SeverityDebug uint16 = 0xE0

// ssDirection describes two-way directions.
// It primarily used in severity order lists.
type ssDirection bool
const Before ssDirection = true
const After ssDirection = false

type LogMsg struct {
	time time.Time
	severity uint16
	content string
	Data interface{}
}

// NewLogMsg allocates and returns a new LogMsg. 
func NewLogMsg() *LogMsg {
	lm := new(LogMsg)
	lm.time = time.Now()
	return lm
}

// Message builds and returns simple message with undefined severity.
func Message(msgFmt string, msgArgs ...interface{}) *LogMsg {
	lm := new(LogMsg)
	lm.time = time.Now()
	lm.content = fmt.Sprintf(msgFmt, msgArgs...)
	return lm
}

// Severity sets severity value for the message.
func (LM *LogMsg) Severity(severity uint16) *LogMsg {
	LM.severity = severity &^ severityShadowMask; return LM
}

// UpdateTime updates message's time to current time.
func (LM *LogMsg) UpdateTime() *LogMsg {
	LM.time = time.Now(); return LM
}

// Add attaches new string to the end of the existing messages text.
func (LM *LogMsg) Add(msgFmt string, msgArgs ...interface{}) *LogMsg {
	LM.content += fmt.Sprintf(msgFmt, msgArgs...); return LM
}

// AddLn adds new string to existing message text as a new line.
func (LM *LogMsg) AddLn(msgFmt string, msgArgs ...interface{}) *LogMsg {
	LM.content += "\n" + fmt.Sprintf(msgFmt, msgArgs...); return LM
}

// Set resets current message's text and sets the given string.
func (LM *LogMsg) Set(msgFmt string, msgArgs ...interface{}) *LogMsg {
	LM.content = fmt.Sprintf(msgFmt, msgArgs...); return LM
}

func (LM *LogMsg) GetTime() time.Time { return LM.time }
func (LM *LogMsg) GetSeverity() uint16 { return LM.severity }
func (LM *LogMsg) GetContent() string { return LM.content }

// -----------------------------------------------------------------------------

type FormatFunc func(*LogMsg) string

type logRecorder interface {
	initialise() error
	//isInitialised() bool
	close()
	write(LogMsg) error
}

type RecorderID string

type Logger struct {
	recorders map[RecorderID]logRecorder

	// determines what severities each recorder will write
	severityMasks map[RecorderID]uint16

	defaults []RecorderID // list of default recorders
	// TODO: description

	// determines severity order for each recorder
	severityOrder map[RecorderID]*list.List
}

// NewLogger allocates and returns a new logger.
func NewLogger() *Logger {
	l := new(Logger)
	l.recorders = make(map[RecorderID]logRecorder)
	l.severityMasks = make(map[RecorderID]uint16)
	l.severityOrder = make(map[RecorderID]*list.List)
	return l
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
	// this function should configure all related fields
	// other functions will panic if they meet a unexpected data

	if L.recorders == nil {
		L.recorders = make(map[RecorderID]logRecorder)
	} else {
		for recID, _ := range L.recorders {
			if recID == id { return false }
		}
	}
	L.recorders[id] = recorder

	// recorder works with all severities by default
	if L.severityMasks == nil {
		L.severityMasks = make(map[RecorderID]uint16)
	}
	L.severityMasks[id] = SeverityAll

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

	// setup severity order for this recorder
	L.severityOrder[id] = list.New().Init()
	// default severity order (up to down)
	L.severityOrder[id].PushBack(Critical)
	L.severityOrder[id].PushBack(Error)
	L.severityOrder[id].PushBack(Warning)
	L.severityOrder[id].PushBack(Notice)
	L.severityOrder[id].PushBack(Info)
	L.severityOrder[id].PushBack(Debug1)
	L.severityOrder[id].PushBack(Debug2)
	L.severityOrder[id].PushBack(Debug3)
	L.severityOrder[id].PushBack(Custom1)
	L.severityOrder[id].PushBack(Custom2)
	L.severityOrder[id].PushBack(Custom3)
	L.severityOrder[id].PushBack(Custom4)

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
	if L.recorders == nil || L.severityMasks == nil || L.severityOrder == nil {
		panic("xlog: bumped to nil")
	}
	if len(L.recorders) == 0 { return NoRecordersError }

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
	if L.recorders == nil || L.severityMasks == nil || L.severityOrder == nil {
		//panic("xlog: bumped to nil")
		return
	}
	if len(L.recorders) == 0 { return }
	for _, rec := range L.recorders {
		rec.close()
	}
}

// AddToDefaults adds given recorders to defaults in that logger.
//
// (The default recorders list determinate which recorders will use for
// writing if custom recorders are not specified in the log message.)
func (L *Logger) AddToDefaults(recorders []RecorderID) error {
	if L.recorders == nil || L.severityMasks == nil || L.severityOrder == nil {
		panic("xlog: bumped to nil")
	}
	if len(L.recorders) == 0 { return NoRecordersError }

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
	if L.recorders == nil || L.severityMasks == nil || L.severityOrder == nil {
		panic("xlog: bumped to nil")
	}
	if len(L.recorders) == 0 { return NoRecordersError }

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

// ChangeSeverityOrder changes severity order for the specified
// recorder. It takes specified flag and moves it before/after
// the target flag position. Only custom flags can be moved.
//
// The function returns nil on success and error overwise.
func (L *Logger) ChangeSeverityOrder(
	recorder RecorderID, srcFlag uint16, dir ssDirection, trgFlag uint16,
) error {

	if len(L.recorders) == 0 { return NoRecordersError }
	if _, exist := L.recorders[recorder]; !exist {
		return RecordersError{ []RecorderID{recorder},
			fmt.Errorf("the recorder '%s' is not registered", recorder),
		}
	}

	// only custom flags are moveable
	newFlag = srcFlag &^ 0xF0FF
	if newFlag == 0 {
		return errors.New("wrong flag value")
	}

	if orderlist, exist := L.severityOrder[recorder]; !exist {
		panic("xlog: recorder id can't be found in severity order map")
	} else {
		var src *list.Element
		var trg *list.Element
		for e := orderlist.Front(); e != nil; e = e.Next() {
			if sev, ok := e.Value.(uint16); !ok {
				panic("xlog: severityOrder, type is invalid")
			} else {
				if sev == srcFlag { src = e
					if trg != nil { break }
				}
				if sev == trgFlag { trg = e
					if src != nil { break }
				}
			}
		}

		// unreachable, all flags should be described
		if src == nil {
			return fmt.Errorf("can't find flag (%b) in the list", srcFlag)
		}
		if trg == nil {
			return fmt.Errorf("can't find flag (%b) in the list", trgFlag)
		}

		// change order
		if dir == Before {
			L.severityOrder[recorder].MoveBefore(src, trg)
		} else { // After
			L.severityOrder[recorder].MoveAfter(src, trg)
		}
	}	

	return nil
}

// SetSeverityMask sets which severities allowed for the given recorder in this logger.
func (L *Logger) SetSeverityMask(recorder RecorderID, flags uint16) error {
	if L.recorders == nil || L.severityMasks == nil || L.severityOrder == nil {
		panic("xlog: bumped to nil")
	}
	if len(L.recorders) == 0 { return NoRecordersError }
	if sevMask, exist := L.severityMasks[recorder]; !exist {
		if _, exist := L.recorders[recorder]; !exist {
			return RecordersError{ []RecorderID{recorder},
				fmt.Errorf("the recorder '%s' is not registered", recorder),
			}
		} else {
			panic("xlog: recorder id can't be found in severity masks map")
		}
		_ = sevMask // THAT'S COMPLETELY STUPID, GOLANG
	} else {
		// zero is allowed (recorder blocked)
		sevMask = flags &^ severityShadowMask
	}
	return nil
}

// Write builds the message with format line and specified severity flag, then calls
// WriteMsg. It allows avoiding calling fmt.Sprintf() function and LogMsg's functions
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
func (L *Logger) WriteMsg(recorders []RecorderID, msg *LogMsg) error {
	if L.recorders == nil || L.severityMasks == nil || L.severityOrder == nil {
		panic("xlog: bumped to nil")
	}
	if len(L.recorders) == 0 { return NoRecordersError }
	if len(L.defaults) == 0 && len(recorders) == 0 {
		return errors.New(
			"the logger has no default recorders, "+
			"but custom recorders are not specified")
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

	(*msg).severity = L.severityProtector((*msg).severity)
	if (*msg).severity == 0 { (*msg).severity = Info }

	for _, recID := range recorders {
		if sevMask, exist := L.severityMasks[recID]; exist {
			if sevMask == 0 { return fmt.Errorf("severity mask is 0") }
			if (*msg).severity == 0 { panic("xlog: severity is 0") }
			if (*msg).severity & sevMask > 0 {
				if rec, exist := L.recorders[recID]; exist {
					if err := rec.write(*msg); err != nil { // <---
						return err // ignore remaining
					}
				} else {
					panic("xlog: recorder id can't be found in registered recorders map")
				}
			}
			} else {
			panic("xlog: recorder id can't be found in severity masks map")
		}
	}

	return nil
}

// This function actually has got a protector role because in some places
// a severity argument should have only one of these flags. So it ensures
// (accordingly to the depth order) that severity value provide only one
// flag.
func (L *Logger) severityProtector(flags uint16) uint16 {
	if L.severityOrder == nil {
		panic("xlog: bumped to nil")
	}
	for e := L.severityOrder.Front(); e != nil; e = e.Next() {
		if sev, ok := e.Value.(uint16); ok {
			if flags & sev > 0 { return sev }
		} else {
			panic("xlog: severityOrder, type is invalid")
		}
	}
	return 0
}
