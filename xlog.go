package xlog

import (
	"container/list"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

/*  xlog message flags

    xxxx xxxx xxxx xxxx
    -+-- --+- ----+----
     |     |      |
 custom    |  default severity flags
 flags  default attributes

custom flags bits:
    --xx : severity
    xx-- : attributes

*/

type MsgFlagT uint16

const ( // severity flags (log level)
	Emerg    MsgFlagT = 0x01 // 0000 0000 0000 0001
	Alert    MsgFlagT = 0x02 // 0000 0000 0000 0010
	Critical MsgFlagT = 0x04 // 0000 0000 0000 0100
	Error    MsgFlagT = 0x08 // 0000 0000 0000 1000
	Warning  MsgFlagT = 0x10 // 0000 0000 0001 0000
	Notice   MsgFlagT = 0x20 // 0000 0000 0010 0000
	Info     MsgFlagT = 0x40 // 0000 0000 0100 0000
	Debug    MsgFlagT = 0x80 // 0000 0000 1000 0000

	CustomB1 MsgFlagT = 0x1000 // 0001 0000 0000 0000
	CustomB2 MsgFlagT = 0x2000 // 0010 0000 0000 0000
)

const ( // attribute flags
	StackTrace      MsgFlagT = 0x100 // 0000 0001 0000 0000
	StackTraceShort MsgFlagT = 0x800 // 0000 1000 0000 0000

	CustomB3 MsgFlagT = 0x4000 // 0100 0000 0000 0000
	CustomB4 MsgFlagT = 0x8000 // 1000 0000 0000 0000
)

func (f MsgFlagT) String() string {
	switch f {
	case Emerg:
		return "EMERG"
	case Alert:
		return "ALERT"
	case Critical:
		return "CRIT"
	case Error:
		return "ERROR"
	case Warning:
		return "WARNING"
	case Notice:
		return "NOTICE"
	case Info:
		return "INFO"
	case Debug:
		return "DEBUG"
	default:
		return fmt.Sprintf("0x%x", int(f))
	}
}

// bit-reset (reversed) mask for severity flags
const SeverityShadowMask MsgFlagT = 0xCF00

// bit-reset (reversed) mask for attribute flags
const AttributeShadowMask MsgFlagT = 0x30FF

// predefined severity sets
const SeverityAll MsgFlagT = 0x30FF
const SeverityMajor MsgFlagT = 0x001F
const SeverityMinor MsgFlagT = 0x00E0
const SeverityDefault MsgFlagT = 0x00FF
const SeverityCustom MsgFlagT = 0x3000

// used when severity is 0
const defaultSeverity = Info

// ssDirection describes two-way directions.
// It primarily used in severity order lists.
type ssDirection bool

const Before ssDirection = true
const After ssDirection = false

func defaultSeverityOrder() *list.List {
	orderlist := list.New().Init()
	orderlist.PushBack(Emerg)
	orderlist.PushBack(Alert)
	orderlist.PushBack(Critical)
	orderlist.PushBack(Error)
	orderlist.PushBack(Warning)
	orderlist.PushBack(Notice)
	orderlist.PushBack(Info)
	orderlist.PushBack(Debug)
	orderlist.PushBack(CustomB1)
	orderlist.PushBack(CustomB2)
	return orderlist
}

// -----------------------------------------------------------------------------

type LogMsg struct {
	time    time.Time
	flags   MsgFlagT
	content string
	Data    interface{}
}

// NewLogMsg allocates and returns a new LogMsg.
func NewLogMsg() *LogMsg {
	lm := new(LogMsg)
	lm.time = time.Now()
	return lm
}

// Message builds and returns simple message with default severity.
func Message(msgFmt string, msgArgs ...interface{}) *LogMsg {
	lm := new(LogMsg)
	lm.time = time.Now()
	lm.content = fmt.Sprintf(msgFmt, msgArgs...)
	return lm
}

// SetFlags sets severity and arrtibute flags for the message.
func (LM *LogMsg) SetFlags(flags MsgFlagT) *LogMsg {
	LM.flags = flags
	return LM
}

// UpdateTime updates message's time to current time.
func (LM *LogMsg) UpdateTime() *LogMsg {
	LM.time = time.Now()
	return LM
}

// Addf attaches new string to the end of the existing message text.
func (LM *LogMsg) Addf(msgFmt string, msgArgs ...interface{}) *LogMsg {
	LM.content += fmt.Sprintf(msgFmt, msgArgs...)
	return LM
}

// Addln_f adds new string to existing message text as a new line.
func (LM *LogMsg) Addf_ln(msgFmt string, msgArgs ...interface{}) *LogMsg {
	LM.content += "\n" + fmt.Sprintf(msgFmt, msgArgs...)
	return LM
}

// Setf resets current message text and sets the given string.
func (LM *LogMsg) Setf(msgFmt string, msgArgs ...interface{}) *LogMsg {
	LM.content = fmt.Sprintf(msgFmt, msgArgs...)
	return LM
}

func (LM *LogMsg) GetTime() time.Time { return LM.time }
func (LM *LogMsg) GetFlags() MsgFlagT { return LM.flags }
func (LM *LogMsg) GetContent() string { return LM.content }

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
	severityMasks map[RecorderID]MsgFlagT

	// determines severity order for each recorder
	severityOrder map[RecorderID]*list.List
}

// NewLogger allocates and returns a new logger.
func NewLogger() *Logger {
	l := new(Logger)
	l.recorders = make(map[RecorderID]logRecorder)
	l.recordersState = make(map[RecorderID]bool)
	l.severityMasks = make(map[RecorderID]MsgFlagT)
	l.severityOrder = make(map[RecorderID]*list.List)
	return l
}

func (L *Logger) NumberOfRecorders() int {
	L.Lock()
	defer L.Unlock()
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

	L.Lock()
	defer L.Unlock()

	if L.recorders == nil {
		L.recorders = make(map[RecorderID]logRecorder)
	} else {
		for recID, _ := range L.recorders {
			if recID == id {
				return ErrWrongRecorderID
			}
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
		L.severityMasks = make(map[RecorderID]MsgFlagT)
	}
	L.severityMasks[id] = SeverityAll

	// check for duplicates
	for i, recID := range L.defaults {
		if recID == id {
			// if id not found in recorders, defaults can't contain it
			//return internalError(ieUnreachable, ".defaults: found unexpected id")
			L.defaults[i] = L.defaults[len(L.defaults)-1]
			L.defaults[len(L.defaults)-1] = RecorderID("")
			L.defaults = L.defaults[:len(L.defaults)-1]
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
	L.severityOrder[id] = defaultSeverityOrder()

	L.initialised = false

	return nil
}

func (L *Logger) UnregisterRecorder(id RecorderID) error {
	L.Lock()
	defer L.Unlock()
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}
	if L.recordersState == nil {
		return internalError(ieCritical, "bumped to nil")
	}

	// close recorder if necessary
	if rec, exist := L.recorders[id]; !exist {
		return ErrWrongRecorderID
	} else {
		if state, exist := L.recordersState[id]; !exist {
			return internalError(ieUnreachable, ".recordersState: missing valid id")
		} else {
			if state {
				rec.close()
			}
		}
	}

	for i, recID := range L.defaults {
		if recID == id {
			L.defaults[i] = L.defaults[len(L.defaults)-1]
			L.defaults[len(L.defaults)-1] = RecorderID("")
			L.defaults = L.defaults[:len(L.defaults)-1]
		}
	}

	delete(L.recorders, id)
	delete(L.recordersState, id)
	delete(L.severityMasks, id)
	delete(L.severityOrder, id)

	return nil
}

// Initialise calls initialisation functions of each registered recorder.
func (L *Logger) Initialise() error {
	L.Lock()
	defer L.Unlock()
	if L.initialised {
		return nil
	} // already initialised
	if L.recorders == nil || L.severityMasks == nil || L.severityOrder == nil {
		return internalError(ieCritical, "bumped to nil")
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}

	br := BatchResult{}
	br.SetMsg("some of the given recorders are not initialised")
	for id, rec := range L.recorders {
		if state, exist := L.recordersState[id]; exist {
			if state {
				continue
			}
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
	L.Lock()
	defer L.Unlock()
	if !L.initialised {
		return
	} // not initialised currently
	if len(L.recorders) == 0 {
		return
	}
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
	L.Lock()
	defer L.Unlock()
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}

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
				continue main_iter // skip this recorder
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
	L.Lock()
	defer L.Unlock()
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}

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
	recorder RecorderID, srcFlag MsgFlagT, dir ssDirection, trgFlag MsgFlagT,
) error {

	L.Lock()
	defer L.Unlock()
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}
	if _, exist := L.recorders[recorder]; !exist {
		return ErrWrongRecorderID
	}
	if L.severityOrder == nil {
		return internalError(ieCritical, "bumped to nil")
	}

	// DISABLED
	//srcFlag = srcFlag &^ 0xCFFF // only custom flags are moveable
	srcFlag = srcFlag &^ severityShadowMask
	trgFlag = trgFlag &^ severityShadowMask
	if srcFlag == 0 {
		return ErrWrongFlagValue
	}
	if trgFlag == 0 {
		return ErrWrongFlagValue
	}
	if srcFlag == trgFlag {
		return ErrWrongFlagValue
	}

	orderlist, exist := L.severityOrder[recorder]
	if !exist {
		return internalError(ieUnreachable, ".severityOrder: missing valid id")
	}
	var src *list.Element
	var trg *list.Element
	for e := orderlist.Front(); e != nil; e = e.Next() {
		if sev, ok := e.Value.(MsgFlagT); !ok {
			return internalError(ieUnreachable, "unexpected value type")
		} else {
			if sev == srcFlag {
				src = e
				if trg != nil {
					break
				}
			}
			if sev == trgFlag {
				trg = e
				if src != nil {
					break
				}
			}
		}
	}

	if src == nil {
		return internalError(ieUnreachable,
			"can't find src flag (%012b)", srcFlag)
	}
	if trg == nil {
		return internalError(ieUnreachable,
			"can't find trg flag (%012b)", trgFlag)
	}

	// change order
	if dir == Before {
		L.severityOrder[recorder].MoveBefore(src, trg)
	} else { // After
		L.severityOrder[recorder].MoveAfter(src, trg)
	}

	return nil
}

// SetSeverityMask sets which severities allowed for the given recorder in this logger.
func (L *Logger) SetSeverityMask(recorder RecorderID, flags MsgFlagT) error {
	L.Lock()
	defer L.Unlock()
	if L.severityMasks == nil {
		return internalError(ieCritical, "bumped to nil")
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}

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

// Write builds the message with format line and specified message flags, then calls
// WriteMsg. It allows avoiding calling fmt.Sprintf() function and LogMsg's functions
// directly, it wraps them. Returns nil in case of success otherwise returns an error.
func (L *Logger) Write(flags MsgFlagT, msgFmt string, msgArgs ...interface{}) error {
	msg := NewLogMsg().SetFlags(flags)
	msg.Setf(msgFmt, msgArgs...)
	return L.WriteMsg(nil, msg)
}

// WriteMsg writes given message using the specified recorders of this logger.
// If custom recorders are not specified, uses default recorders. Returns nil
// on success and error on fail.
func (L *Logger) WriteMsg(recorders []RecorderID, msg *LogMsg) error {
	L.Lock()
	defer L.Unlock()
	if !L.initialised {
		return ErrNotInitialised
	}
	if L.severityMasks == nil || L.severityOrder == nil {
		return internalError(ieCritical, "bumped to nil")
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}
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

	if (*msg).flags&StackTraceShort > 0 {
		st := debug.Stack()
		str := "---------- stack trace ----------"
		lines := strings.Split(string(st), "\n")
		// select strings
		var accumulator []string
		for i := 1; i < len(lines)-1; i += 2 {
			accumulator = append(accumulator, lines[i])
		}
		// make a result
		str += "   " + lines[0] + "\n"
		for i := 1; i < len(accumulator); i++ {
			str += accumulator[i] + "\n"
		}
		str += "---------------------------------"
		(*msg).content += "\n" + str
	} else if (*msg).flags&StackTrace > 0 {
		str := "---------- stack trace ----------"
		str += "   " + string(debug.Stack())
		str += "---------------------------------"
		(*msg).content += "\n" + str
	}

	//severity := (*msg).flags &^ severityShadowMask
	//attirbutes := (*msg).flags &^ attributeShadowMask
	if (*msg).flags&^severityShadowMask == 0 {
		(*msg).flags |= defaultSeverity
	}

	for _, recID := range recorders {
		ie := L.severityProtector(L.severityOrder[recID], &((*msg).flags))
		if ie != nil {
			br.Fail(recID, ie)
			continue
		}
		if sevMask, exist := L.severityMasks[recID]; exist {
			/* already checked
			if (*msg).flags &^ severityShadowMask == 0 {
				ie = internalError(ieUnreachable, "severity is 0")
				br.Fail(recID, ie)
				continue
			} */
			if ((*msg).flags&^severityShadowMask)&sevMask > 0 { // severity allowed
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
func (L *Logger) severityProtector(orderlist *list.List, flags *MsgFlagT) error { // NO LOCK
	if orderlist == nil || orderlist.Len() == 0 {
		return internalError(ieCritical, "wrong 'orderlist' parameter value")
	}
	for e := orderlist.Front(); e != nil; e = e.Next() {
		if sev, ok := e.Value.(MsgFlagT); ok {
			if (*flags&^severityShadowMask)&sev > 0 {
				*flags = *flags &^ (^severityShadowMask) // reset
				*flags = *flags | sev                    // set
				return nil
			}
		} else {
			return internalError(ieUnreachable, "type is invalid")
		}
	}
	return internalError(ieUnreachable, "can't find severity flag in orderlist (%012b)", *flags)
}
