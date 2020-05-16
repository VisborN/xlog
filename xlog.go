package xlog

import (
	"container/list"
	"fmt"
	"runtime"
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

var GlobalDisable bool = false

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

// Addfn adds new string to existing message text as a new line.
func (LM *LogMsg) Addfn(msgFmt string, msgArgs ...interface{}) *LogMsg {
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

type ControlSignal string

const (
	SignalInit  ControlSignal = "SIG_INIT"
	SignalClose ControlSignal = "SIG_CLOSE"
	SignalStop  ControlSignal = "SIG_STOP"
)

type FormatFunc func(*LogMsg) string

type logRecorder interface {
	Listen()
	GetChannels() ChanBundle
}

type RecorderID string

// ChanBundle structure represents recorder interface channels.
type ChanBundle struct {
	ChCtl     chan<- ControlSignal
	ChMsg     chan<- LogMsg
	ChSyncErr <-chan error
}

type Logger struct {
	sync.Mutex

	initialised bool
	// We must sure that functions init/close doesn't call successively.
	// Otherwise we can have problems with reference counters calculations.

	recorders map[RecorderID]ChanBundle

	// describes initialisation status of each recorder
	recordersState map[RecorderID]bool

	defaults []RecorderID // list of default recorders
	// Default recorders used for writing by default
	// if custom recorders are not specified (nil).

	// determines what severities each recorder will write
	severityMasks map[RecorderID]MsgFlagT

	// determines the severity order for each recorder
	severityOrder map[RecorderID]*list.List
}

// NewLogger allocates and returns a new logger.
func NewLogger() *Logger {
	l := new(Logger)
	l.recorders = make(map[RecorderID]ChanBundle)
	l.recordersState = make(map[RecorderID]bool)
	l.severityMasks = make(map[RecorderID]MsgFlagT)
	l.severityOrder = make(map[RecorderID]*list.List)
	return l
}

// DEBUG FUNC
func (L *Logger) NumberOfRecorders() int {
	return len(L.recorders)
}

// The same as RegisterRecorderEx, but adds recorder to defaults anyways.
func (L *Logger) RegisterRecorder(id RecorderID, channels ChanBundle) error {
	return L.RegisterRecorderEx(id, channels, true)
}

// RegisterRecorder registers the recorder in the logger with the given id.
// asDefault parameter says whether the need to set it as default recorder.
func (L *Logger) RegisterRecorderEx(id RecorderID, intrf ChanBundle, asDefault bool) error {
	// This function should configure all related fields. Other functions
	// will return an error or cause panic if they meet a wrong logger data.

	if GlobalDisable {
		return nil
	}

	L.Lock()
	defer L.Unlock()

	if L.recorders == nil {
		// We should provide Logger as exported type. So, these checks
		// are necessary in case if object was created w/o New Logger()
		L.recorders = make(map[RecorderID]ChanBundle)
	} else {
		for recID, _ := range L.recorders {
			if recID == id {
				return ErrWrongRecorderID
			}
		}
	}
	L.recorders[id] = intrf

	// setup initialisation state
	if L.recordersState == nil {
		L.recordersState = make(map[RecorderID]bool)
	}
	L.recordersState[id] = false

	// setup default severity mask
	if L.severityMasks == nil {
		L.severityMasks = make(map[RecorderID]MsgFlagT)
	}
	L.severityMasks[id] = SeverityAll // works with all severities by default

	// check for duplicates
	// TODO: a bit thick check, remove ?
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
	if GlobalDisable {
		return nil
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}
	if L.recordersState == nil {
		return internalError(ieCritical, "bumped to nil")
	}

	// close recorder if necessary
	if rc, exist := L.recorders[id]; !exist {
		return ErrWrongRecorderID
	} else {
		if initialised, exist := L.recordersState[id]; !exist {
			return internalError(ieUnreachable, ".recordersState: missing valid id")
		} else {
			if initialised {
				rc.ChCtl <- SignalClose
			}
		}
	}

	L.Lock()

	// remove from defaults
	for i, recID := range L.defaults {
		if recID == id {
			L.defaults[i] = L.defaults[len(L.defaults)-1]
			L.defaults[len(L.defaults)-1] = RecorderID("")
			L.defaults = L.defaults[:len(L.defaults)-1]
			break // duplicates ain't possible
		}
	}

	if len(L.recorders) == 1 {
		L.initialised = false
	}

	delete(L.recorders, id)
	delete(L.recordersState, id)
	delete(L.severityMasks, id)
	delete(L.severityOrder, id)

	L.Unlock()

	return nil
}

// Initialise invokes initialisation functions of each registered recorder.
func (L *Logger) Initialise() error {
	if GlobalDisable {
		return nil
	}
	if L.initialised {
		return nil
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}
	if L.recorders == nil || L.severityMasks == nil || L.severityOrder == nil {
		return internalError(ieCritical, "bumped to nil")
	}

	L.Lock()
	defer L.Unlock()

	br := BatchResult{}
	br.SetMsg("some of the given recorders are not initialised")
	for id, rec := range L.recorders {
		if initialised, exist := L.recordersState[id]; exist {
			if initialised {
				continue
			}
			rec.ChCtl <- SignalInit
			/// response on initialisation error ///
			err := <-rec.ChSyncErr
			if err != nil {
				// ACTIONS...
				br.Fail(id, err)
			} else {
				L.recordersState[id] = true
				br.OK(id)
			}
		} else {
			// recorder is registered but id is missing in states map
			br.Fail(id, internalError(ieUnreachable,
				".recordersState: missing valid id"))
		}
	}

	/*** LOGIC AT PARTIAL INITIALISATION ***/
	if br.Errors() != nil {
		return br
	} else {
		// all recorders should be initialised for success state
		L.initialised = true
		return nil
	}
}

// Close invokes closing functions of each registered recorder.
func (L *Logger) Close() {
	if !L.initialised {
		return
	}
	if len(L.recorders) == 0 {
		return
	}
	for _, rec := range L.recorders {
		rec.ChCtl <- SignalClose
	}

	// {src: t2u-race-2be43}
	runtime.Gosched() // wait other WriteMsg operations

	L.Lock()
	L.initialised = false
	L.Unlock()
}

// AddToDefaults sets given recorders as default for that logger.
func (L *Logger) AddToDefaults(recorders []RecorderID) error {
	if GlobalDisable {
		return nil
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}

	L.Lock()
	defer L.Unlock()

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
		// set as default
		L.defaults = append(L.defaults, recID)
		br.OK(recID)
	}

	if br.Errors() != nil {
		return br
	}
	return nil
}

// RemoveFromDefaults removes given recorders form defaults in that logger.
func (L *Logger) RemoveFromDefaults(recorders []RecorderID) error {
	if GlobalDisable {
		return nil
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}

	L.Lock()
	defer L.Unlock()

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
// the target flag position.
//
// Only custom flags can be moved (currently disabled).
//
// The function returns nil on success and error overwise.
func (L *Logger) ChangeSeverityOrder(
	recorder RecorderID, srcFlag MsgFlagT, dir ssDirection, trgFlag MsgFlagT,
) error {
	if GlobalDisable {
		return nil
	}

	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}
	if _, exist := L.recorders[recorder]; !exist {
		return ErrWrongRecorderID
	}
	if L.severityOrder == nil {
		return internalError(ieCritical, "bumped to nil")
	}

	// DISABLED CURRENTLY (TODO)
	//srcFlag = srcFlag &^ 0xCFFF // only custom flags are moveable
	srcFlag = srcFlag &^ SeverityShadowMask
	trgFlag = trgFlag &^ SeverityShadowMask
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

	L.Lock()

	// change order
	if dir == Before {
		L.severityOrder[recorder].MoveBefore(src, trg)
	} else { // After
		L.severityOrder[recorder].MoveAfter(src, trg)
	}

	L.Unlock()
	return nil
}

// SetSeverityMask sets which severities allowed for the given recorder in this logger.
func (L *Logger) SetSeverityMask(recorder RecorderID, flags MsgFlagT) error {
	if GlobalDisable {
		return nil
	}
	if L.severityMasks == nil {
		return internalError(ieCritical, "bumped to nil")
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}

	if sevMask, exist := L.severityMasks[recorder]; !exist {
		// already failed in this case, we should choose error here
		if _, exist := L.recorders[recorder]; !exist {
			return ErrWrongRecorderID
		} else {
			return internalError(ieUnreachable, ".severityMasks: missing valid id")
		}
		_ = sevMask // THAT'S COMPLETELY STUPID, GOLANG
	} else {
		L.Lock()

		// zero is allowed (recorder blocked)
		L.severityMasks[recorder] = flags &^ SeverityShadowMask

		L.Unlock()
	}

	return nil
}

// Write builds the message with format line and specified message flags, then calls
// WriteMsg. It allows avoiding calling fmt.Sprintf() function and LogMsg's functions
// directly, it wraps them.
//
// Returns nil in case of success otherwise returns an error.
func (L *Logger) Write(flags MsgFlagT, msgFmt string, msgArgs ...interface{}) error {
	if GlobalDisable {
		return nil
	}
	msg := NewLogMsg().SetFlags(flags)
	msg.Setf(msgFmt, msgArgs...)
	return L.WriteMsg(nil, msg)
}

// {Logger}: only read access
//
// WriteMsg send write signal with given message to the specified recorders.
// If custom recorders are not specified, uses default recorders of this logger.
//
// Returns nil on success and error on fail.
func (L *Logger) WriteMsg(recorders []RecorderID, msg *LogMsg) error {
	if GlobalDisable {
		return nil
	}

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
		// CAREFULLY! DON'T DELETE THAT
		// This check is valid, that's not L.recorders.
		return ErrNotWhereToWrite
	}

	br := BatchResult{}
	br.SetMsg("an error occurred in some of the given recorders")

	// set target recorders to write
	if len(recorders) > 0 {
		// if custom recorders specified, check em for valid
		for i, recID := range recorders {
			if _, exist := L.recorders[recID]; !exist {
				br.Fail(recID, ErrWrongRecorderID)
				// remove it from the list
				recorders[i] = recorders[len(recorders)-1]
				recorders[len(recorders)-1] = ""
				recorders = recorders[:len(recorders)-1]
			}
		}
	} else { // use default recorders
		recorders = L.defaults
	}

	// add stack trace info if the flags specified
	if (*msg).flags&StackTraceShort > 0 {
		// TODO: more flexible way
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

	// check that severity flag specified
	if (*msg).flags&^SeverityShadowMask == 0 {
		(*msg).flags |= defaultSeverity
	}

	for _, recID := range recorders {
		if err := L.severityProtector(L.severityOrder[recID], &((*msg).flags)); err != nil {
			br.Fail(recID, err)
			continue
		}
		if sevMask, exist := L.severityMasks[recID]; exist {
			/* already checked
			if (*msg).flags &^ SeverityShadowMask == 0 {
				br.Fail(recID, internalError(ieUnreachable, "severity is 0"))
				continue
			} */
			if ((*msg).flags&^SeverityShadowMask)&sevMask > 0 { // severity filter
				rec := L.recorders[recID] // recorder id is valid, already checked

				rec.ChMsg <- *msg
				br.OK(recID)
				// NO ERROR CHECK
			}
		} else {
			br.Fail(recID,
				internalError(ieUnreachable, ".severityMasks: missing valid id"))
		}
	}

	// write errors ain't possible currently
	//if br.Errors() != nil { return br }
	return nil
}

// This function actually has got a protector role because in some places
// a severity argument should have only one of these flags. So it ensures
// (accordingly to the depth order) that severity value provide only one
// flag.
func (L *Logger) severityProtector(orderlist *list.List, flags *MsgFlagT) error {
	if orderlist == nil || orderlist.Len() == 0 {
		return internalError(ieCritical, "wrong 'orderlist' parameter value")
	}
	for e := orderlist.Front(); e != nil; e = e.Next() {
		if sev, ok := e.Value.(MsgFlagT); ok {
			if (*flags&^SeverityShadowMask)&sev > 0 {
				*flags = *flags &^ (^SeverityShadowMask) // reset
				*flags = *flags | sev                    // set
				return nil
			}
		} else {
			return internalError(ieUnreachable, "type is invalid")
		}
	}
	return internalError(ieUnreachable, "can't find severity flag in orderlist (%012b)", *flags)
}
