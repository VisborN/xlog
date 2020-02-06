package xlog

import (
	"fmt"
	"time"
	"sync"
	"container/list"
)

/*  xlog message flags

    xxxx xxxx xxxx xxxx
    -+-- --+- ----+----
     |     |      |
attributes |  defeult severity flags
        custom severity flags
*/

type SevFlagT uint16

const ( // severity flags (log level)
	Critical SevFlagT = 0x01  // 0000 0000 0000 0001
	Error    SevFlagT = 0x02  // 0000 0000 0000 0010
	Warning  SevFlagT = 0x04  // 0000 0000 0000 0100
	Notice   SevFlagT = 0x08  // 0000 0000 0000 1000
	Info     SevFlagT = 0x10  // 0000 0000 0001 0000
	Debug1   SevFlagT = 0x20  // 0000 0000 0010 0000
	Debug2   SevFlagT = 0x40  // 0000 0000 0100 0000
	Debug3   SevFlagT = 0x80  // 0000 0000 1000 0000

	Custom1  SevFlagT = 0x100 // 0000 0001 0000 0000
	Custom2  SevFlagT = 0x200 // 0000 0010 0000 0000
	Custom3  SevFlagT = 0x300 // 0000 0100 0000 0000
	Custom4  SevFlagT = 0x400 // 0000 1000 0000 0000
)

func (S SevFlagT) String() string {
	switch S {
	case Critical: return "CRIT"
	case Error:    return "ERROR"
	case Warning:  return "WARNING"
	case Notice:   return "NOTICE"
	case Info:     return "INFO"
	case Debug1:   return "DEBUG-1"
	case Debug2:   return "DEBUG-2"
	case Debug3:   return "DEBUG-3"
	default:
		return fmt.Sprintf("0x%x", int(S))
	}
}

// bit-reset (reversed) mask for severity flags
const severityShadowMask SevFlagT = 0xF000
// bit-reset (reversed) mask for attribute flags
const attributeShadowMask SevFlagT = 0x0FFF

// predefined severity sets
const SeverityAll    SevFlagT = 0xFFF
const SeverityMajor  SevFlagT = 0x00F
const SeverityMinor  SevFlagT = 0x0F0
const SeverityDebug  SevFlagT = 0x0E0
const SeverityCustom SevFlagT = 0xF00

// ssDirection describes two-way directions.
// It primarily used in severity order lists.
type ssDirection bool
const Before ssDirection = true
const After ssDirection = false

type LogMsg struct {
	time time.Time
	severity SevFlagT
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
func (LM *LogMsg) Severity(severity SevFlagT) *LogMsg {
	LM.severity = severity &^ severityShadowMask; return LM
}

// UpdateTime updates message's time to current time.
func (LM *LogMsg) UpdateTime() *LogMsg {
	LM.time = time.Now(); return LM
}

// Add attaches new string to the end of the existing message text.
func (LM *LogMsg) Addf(msgFmt string, msgArgs ...interface{}) *LogMsg {
	LM.content += fmt.Sprintf(msgFmt, msgArgs...); return LM
}

// Addln adds new string to existing message text as a new line.
func (LM *LogMsg) Addf_ln(msgFmt string, msgArgs ...interface{}) *LogMsg {
	LM.content += "\n" + fmt.Sprintf(msgFmt, msgArgs...); return LM
}

// Set resets current message text and sets the given string.
func (LM *LogMsg) Setf(msgFmt string, msgArgs ...interface{}) *LogMsg {
	LM.content = fmt.Sprintf(msgFmt, msgArgs...); return LM
}

func (LM *LogMsg) GetTime()     time.Time { return LM.time }
func (LM *LogMsg) GetSeverity() SevFlagT  { return LM.severity }
func (LM *LogMsg) GetContent()  string    { return LM.content }

// -----------------------------------------------------------------------------

type FormatFunc func(*LogMsg) string

type logRecorder interface {
	initialise() error
	close()
	write(LogMsg) error
}

type RecorderID string

type Logger struct {
	sync.Mutex

	initialised bool // init falg
	// We must sure that functions init/close doesn't call successively.
	// In any other case, we can have problems with reference counters calculations.

	recorders map[RecorderID]logRecorder

	// describes initialisation status of each recorder
	recordersState map[RecorderID]bool

	defaults []RecorderID // list of default recorders
	// TODO: description

	// determines what severities each recorder will write
	severityMasks map[RecorderID]SevFlagT	

	// determines severity order for each recorder
	severityOrder map[RecorderID]*list.List
}

// NewLogger allocates and returns a new logger.
func NewLogger() *Logger {
	l := new(Logger)
	l.recorders = make(map[RecorderID]logRecorder)
	l.recordersState = make(map[RecorderID]bool)
	l.severityMasks = make(map[RecorderID]SevFlagT)
	l.severityOrder = make(map[RecorderID]*list.List)
	return l
}

func (L *Logger) NumberOfRecorders() int {
	L.Lock(); defer L.Unlock()
	return len(L.recorders)
}

// The same as RegisterRecorderEx, but adds recorder to defaults automatically.
func (L *Logger) RegisterRecorder(id RecorderID, recorder logRecorder) error {
	return L.RegisterRecorderEx(id, true, recorder)
}

// RegisterRecorder registers the recorder in the logger with the given id.
// An asDefault parameter says whether the need to set it as default recorder.
func (L *Logger) RegisterRecorderEx(id RecorderID, asDefault bool, recorder logRecorder) error {
	// This function should configure all related fields. Other functions
	// will return critical error if they meet a wrong logger data.

	L.Lock(); defer L.Unlock()

	if L.recorders == nil {
		L.recorders = make(map[RecorderID]logRecorder)
	} else {
		for recID, _ := range L.recorders {
			if recID == id { return ErrWrongRecorderID }
		}
	}
	L.recorders[id] = recorder

	// setup initialisation state
	if L.recordersState == nil {
		L.recordersState = make(map[RecorderID]bool)
	}
	L.recordersState[id] = false

	// recorder works with all severities by default
	if L.severityMasks == nil {
		L.severityMasks = make(map[RecorderID]SevFlagT)
	}
	L.severityMasks[id] = SeverityAll

	// check for ensure that it's correct
	for _, recID := range L.defaults {
		if recID == id {
			// if id not found in recorders, defaults can't contain it
			return internalError(ieUnreachable, ".defaults: found unexpected id")
		}
	}

	// set as default if necessary
	if asDefault {
		L.defaults = append(L.defaults, id)
	}

	// setup severity order for this recorder
	if L.severityOrder == nil {
		L.severityOrder = make(map[RecorderID]*list.List)
	}
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

	return nil
}

// Initialise calls initialisation functions of each registered recorder.
func (L *Logger) Initialise() error {
	L.Lock(); defer L.Unlock()
	if L.initialised { return nil } // already initialised
	if L.recorders == nil || L.severityMasks == nil || L.severityOrder == nil {
		return internalError(ieCritical, "bumped to nil")
	}
	if len(L.recorders) == 0 { return ErrNoRecorders }

	br := BatchResult{}
	br.SetMsg("some of the given recorders are not initialised")
	for id, rec := range L.recorders {
		if state, exist := L.recordersState[id]; exist {
			if state { continue }
			if err := rec.initialise(); err != nil {
				br.Fail(id, err)
			} else {
				L.recordersState[id] = true
				br.OK(id)
			}
		} else {
			br.Fail(id, internalError(ieUnreachable,
				".recordersState: missing valid id"))
		}
	}
	if br.Errors() != nil {
		return br
	}
	L.initialised = true
	return nil
}

// Close calls closing functions of each registered recorder.
func (L *Logger) Close() {
	L.Lock(); defer L.Unlock()
	if !L.initialised { return } // not initialised currently
	if len(L.recorders) == 0 { return }
	for _, rec := range L.recorders {
		rec.close()
	}
	L.initialised = false
}

// AddToDefaults adds given recorders to defaults in that logger.
//
// (The default recorders list determinate which recorders will use for
// writing if custom recorders are not specified in the log message.)
func (L *Logger) AddToDefaults(recorders []RecorderID) error {
	L.Lock(); defer L.Unlock()
	if len(L.recorders) == 0 { return ErrNoRecorders }

	// check registered recorders
	br := BatchResult{}
	br.SetMsg("some of given recorder IDs are invalid")
	for i, recID := range recorders {
		if _, exist := L.recorders[recID]; !exist { // not found
			br.Fail(recID, ErrWrongRecorderID)
			// remove item from list
			recorders[i] = recorders[len(recorders)-1]
			recorders[len(recorders)-1] = ""
			recorders = recorders[:len(recorders)-1]
		}
	}

main_iter:
	for _, recID := range recorders {
		// check defaults for duplicate
		for _, defID := range L.defaults {
			if defID == recID { // id already in the defaults
				continue main_iter; // skip this recorder
			}
		}
		// register as default
		L.defaults = append(L.defaults, recID)
		br.OK(recID)
	}

	if br.Errors() != nil {
		return br
	}
	return nil
}

// RemoveFromDefaults removes given recorders form defaults in that logger.
//
// (The default recorders list determinate which recorders will use for
// writing if custom recorders are not specified in the log message.)
func (L *Logger) RemoveFromDefaults(recorders []RecorderID) error {
	L.Lock(); defer L.Unlock()
	if len(L.recorders) == 0 { return ErrNoRecorders }

	// check registered recorders
	br := BatchResult{}
	br.SetMsg("some of given recorder IDs are invalid")
	for i, recID := range recorders {
		if _, exist := L.recorders[recID]; !exist { // not found
			br.Fail(recID, ErrWrongRecorderID)
			// remove item from list
			recorders[i] = recorders[len(recorders)-1]
			recorders[len(recorders)-1] = RecorderID("")
			recorders = recorders[:len(recorders)-1]
		}
	}	

	// delete given ids from defaults
	for _, recID := range recorders {
		for i, defID := range L.defaults {
			if defID == recID {
				L.defaults[i] = L.defaults[len(L.defaults)-1]
				L.defaults[len(L.defaults)-1] = RecorderID("")
				L.defaults = L.defaults[:len(L.defaults)-1]
				br.OK(recID)
			}
		}
	}

	if br.Errors() != nil {
		return br
	}
	return nil
}

// ChangeSeverityOrder changes severity order for the specified
// recorder. It takes specified flag and moves it before/after
// the target flag position. Only custom flags can be moved.
//
// The function returns nil on success and error overwise.
func (L *Logger) ChangeSeverityOrder(
	recorder RecorderID, srcFlag SevFlagT, dir ssDirection, trgFlag SevFlagT,
) error {
	
	L.Lock(); defer L.Unlock()
	if len(L.recorders) == 0 { return ErrNoRecorders }
	if _, exist := L.recorders[recorder]; !exist {
		return ErrWrongRecorderID
	}		
	if L.severityOrder == nil {
		return internalError(ieCritical, "bumped to nil")
	}

	// DISABLED
	//srcFlag = srcFlag &^ 0xF0FF // only custom flags are moveable

	if srcFlag == 0 { return ErrWrongFlagValue }	
		
	orderlist, exist := L.severityOrder[recorder]
	if !exist {
		return internalError(ieUnreachable, ".severityOrder: missing valid id")
	}
	var src *list.Element
	var trg *list.Element
	for e := orderlist.Front(); e != nil; e = e.Next() {
		if sev, ok := e.Value.(SevFlagT); !ok {
			return internalError(ieUnreachable, "unexpected value type")
		} else {
			if sev == srcFlag { src = e
				if trg != nil { break }
			}
			if sev == trgFlag { trg = e
				if src != nil { break }
			}
		}
	}
	
	if src == nil { return ErrWrongFlagValue }
	if trg == nil { return ErrWrongFlagValue }
	
	// change order
	if dir == Before {
		L.severityOrder[recorder].MoveBefore(src, trg)
	} else { // After
		L.severityOrder[recorder].MoveAfter(src, trg)
	}

	return nil
}

// SetSeverityMask sets which severities allowed for the given recorder in this logger.
func (L *Logger) SetSeverityMask(recorder RecorderID, flags SevFlagT) error {
	L.Lock(); defer L.Unlock()
	if L.severityMasks == nil { return internalError(ieCritical, "bumped to nil") }
	if len(L.recorders) == 0 { return ErrNoRecorders }

	if sevMask, exist := L.severityMasks[recorder]; !exist {
		// already failed, we should choose error here
		if _, exist := L.recorders[recorder]; !exist {
			return ErrWrongRecorderID
		} else {
			return internalError(ieUnreachable, ".severityMasks: missing valid id")
		}
		_ = sevMask // THAT'S COMPLETELY STUPID, GOLANG
	} else {
		// zero is allowed (recorder blocked)
		L.severityMasks[recorder] = flags &^ severityShadowMask
	}

	return nil
}

// Write builds the message with format line and specified severity flag, then calls
// WriteMsg. It allows avoiding calling fmt.Sprintf() function and LogMsg's functions
// directly, it wraps them. Returns nil in case of success otherwise returns an error.
func (L *Logger) Write(severity SevFlagT, msgFmt string, msgArgs ...interface{}) error {
	msg := NewLogMsg().Severity(severity)
	msg.Setf(msgFmt, msgArgs...)
	return L.WriteMsg(nil, msg)
}

// WriteMsg writes given message using the specified recorders of this logger.
// If custom recorders are not specified, uses default recorders. Returns nil
// on success and error on fail.
func (L *Logger) WriteMsg(recorders []RecorderID, msg *LogMsg) error {
	L.Lock(); defer L.Unlock()
	if !L.initialised { return ErrNotInitialised }
	if L.severityMasks == nil || L.severityOrder == nil {
		return internalError(ieCritical, "bumped to nil") }
	if len(L.recorders) == 0 { return ErrNoRecorders }
	if len(L.defaults) == 0 && len(recorders) == 0 {
		return ErrNotWhereToWrite
	}

	br := BatchResult{}
	br.SetMsg("an error occurred in some of the given recorders")

	if len(recorders) > 0 { // custom rec. specified
		for i, recID := range recorders {
			if _, exist := L.recorders[recID]; !exist {
				br.Fail(recID, ErrWrongRecorderID)
				// remove item from list
				recorders[i] = recorders[len(recorders)-1]
				recorders[len(recorders)-1] = ""
				recorders = recorders[:len(recorders)-1]
			}
		}
	} else { // use default recorders
		recorders = L.defaults
	}

	for _, recID := range recorders {
		if (*msg).severity == 0 { (*msg).severity = Info } // <---------------------+
		ie := L.severityProtector(L.severityOrder[recID], &((*msg).severity) ) //   |
		if ie != nil {                                                         //   |
			br.Fail(recID, ie);                                                  //   |
			continue                                                             //   |
		}                                                                      //   |
		if sevMask, exist := L.severityMasks[recID]; exist {                   //   |
			//if (*msg).severity == 0 { // <------------------------------------------+
			//	ie = internalError(ieUnreachable, "severity is 0")
			//	br.Fail(recID, ie)
			//	continue
			//}
			if (*msg).severity & sevMask > 0 { // severity allowed
				rec := L.recorders[recID] // recorder id is valid, already checked
				if err := rec.write(*msg); err != nil {
					br.Fail(recID, err)
				} else {
					br.OK(recID)
				}
			}
		} else {
			ie = internalError(ieUnreachable, ".severityMasks: missing valid id")
			br.Fail(recID, ie)
		}
	}

	if br.Errors() != nil {
		return br
	}
	return nil
}

// This function actually has got a protector role because in some places
// a severity argument should have only one of these flags. So it ensures
// (accordingly to the depth order) that severity value provide only one
// flag.
func (L *Logger) severityProtector(orderlist *list.List, flags *SevFlagT) error { // NO LOCK
	if orderlist == nil || orderlist.Len() == 0 {
		return internalError(ieCritical, "wrong 'orderlist' parameter value")
	}
	for e := orderlist.Front(); e != nil; e = e.Next() {
		if sev, ok := e.Value.(SevFlagT); ok {
			if *flags & sev > 0 {
				*flags = sev
				return nil
			}
		} else {
			return internalError(ieUnreachable, "type is invalid")
		}
	}
	return internalError(ieUnreachable, "can't find severity flag in orderlist (%012b)", *flags)
}
