package xlog

import (
	"container/list"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/rs/xid"
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

// MsgFlagT type represents messages bit flags for severities and attributes.
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

// bit-reset (reversed) mask for severity flags
const SeverityShadowMask MsgFlagT = 0xCF00

// bit-reset (reversed) mask for attribute flags
const AttributeShadowMask MsgFlagT = 0x30FF

const ( // predefined severity sets
	SeverityAll     MsgFlagT = 0x30FF // Default | Custom
	SeverityMajor   MsgFlagT = 0x001F // Major = Emerg | Alert | Critical | Error | Warning
	SeverityMinor   MsgFlagT = 0x00E0 // Minor = Notice | Info
	SeverityDefault MsgFlagT = 0x00FF // Default = Major | Minor | Debug
	SeverityCustom  MsgFlagT = 0x3000 // Custom = CustomB1 | CustomB2
)

// used when severity is 0
const defaultSeverity = Info

// Returns string with severity name in text format.
// For unexpected flags returns string with hexadecimal code.
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

// ssDirection describes two-way directions.
// It primarily used in severity order lists.
type ssDirection bool

const Before ssDirection = true
const After ssDirection = false

type ListOfRecorders []LogRecorder

func (list *ListOfRecorders) Add(rec LogRecorder) {
	if rec != nil {
		*list = append(*list, rec)
	}
}

func (list ListOfRecorders) FindByID(id xid.ID) LogRecorder {
	for i, rec := range list {
		if rec == nil {
			list[i] = list[len(list)-1]
			list[len(list)-1] = nil
			list = list[:len(list)-1]
		}
		if rec.GetID().Compare(id) == 0 {
			return rec
		}
	}
	return nil
}

// ---------------------------------------- CONFIG

type bool_s struct {
	sync.RWMutex
	v bool
}

func (m *bool_s) Set(value bool) {
	m.Lock()
	defer m.Unlock()
	m.v = value
}

func (m *bool_s) Get() bool {
	m.RLock()
	defer m.RUnlock()
	return m.v
}

// If true, all Logger methods will be skipped.
//
//   default value: false
var CfgGlobalDisable bool_s = bool_s{v: false}

// If true, Initialise function with passed 'objects' argument
// will start listeners by self for not-listening recorders.
//
//   default value: true
var CfgAutoStartListening bool_s = bool_s{v: true}

// -----------------------------------------------------------------------------

// LogMsg represents a log message. It contains message data,
// flags, time and extra data for non-default handling.
type LogMsg struct {
	time    time.Time
	flags   MsgFlagT
	content string
	Data    interface{} // extra data
}

// NewLogMsg allocates and returns a new LogMsg.
func NewLogMsg() *LogMsg {
	lm := new(LogMsg)
	lm.time = time.Now()
	return lm
}

// Message builds and returns simple message with default severity (0).
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

// Addfn adds new string to the existing message text as a new line.
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

type signalType string

type controlSignal struct {
	stype signalType
	data  interface{}
}

const (
	SigInit  signalType = "SIG_INIT"
	SigClose signalType = "SIG_CLOSE"
	SigStop  signalType = "SIG_STOP"

	SigSetErrChan  signalType = "SIG_SET_ERR"
	SigSetDbgChan  signalType = "SIG_SET_DBG"
	SigDropErrChan signalType = "SIG_DROP_ERR"
	SigDropDbgChan signalType = "SIG_GROP_DBG"
)

func SignalInit(chErr chan error) controlSignal         { return controlSignal{SigInit, chErr} }
func SignalClose() controlSignal                        { return controlSignal{SigClose, nil} }
func SignalStop() controlSignal                         { return controlSignal{SigStop, nil} }
func SignalSetErrChan(chErr chan<- error) controlSignal { return controlSignal{SigSetErrChan, chErr} }
func SignalSetDbgChan(chDbg chan<- debugMessage) controlSignal {
	return controlSignal{SigSetDbgChan, chDbg}
}
func SignalDropErrChan() controlSignal { return controlSignal{SigDropErrChan, nil} }
func SignalDropDbgChan() controlSignal { return controlSignal{SigDropDbgChan, nil} }

// FormatFunc is an interface for the recorder's format function. This
// function handles the log message object and returns final output string.
type FormatFunc func(*LogMsg) string

// LogRecorder is an interface for the log endpoint recorder. These types
// should provide write functions and other methods to correctly interact
// with a logging destination objects (e.g. files, streams etc).
type LogRecorder interface {
	Listen()
	IsListening() bool
	Intrf() RecorderInterface
	GetID() xid.ID
}

// RecorderInterface structure represents interface channels of the recorder.
type RecorderInterface struct {
	ChCtl chan<- controlSignal
	ChMsg chan<- LogMsg
	id    xid.ID
}

// RecorderID is an identifier used in Logger functions to select recorders.
type RecorderID string

type Logger struct {
	sync.RWMutex

	initialised bool // true only if all recorders have been initialised
	// We must sure that functions init/close doesn't call successively.
	// Otherwise we can have problems with reference counters calculations.

	recorders     map[RecorderID]RecorderInterface
	recordersInit map[RecorderID]bool
	//recordersXID  map[RecorderID]xid.ID

	defaults []RecorderID // list of default recorders
	// Default recorders used for writing by default
	// if custom recorders are not specified (nil).

	// determines what severities each recorder will write
	severityMasks map[RecorderID]MsgFlagT

	// determines the severity order for each recorder
	severityOrder map[RecorderID]*list.List

	// it used for tests, shouldn't be exported or documented
	_falseInit _recList
}

// it used for tests, shouldn't be exported or documented
type _recList []RecorderID

func (r *_recList) add(rec RecorderID) {
	*r = append(*r, rec)
}

func (r *_recList) check(rec RecorderID) bool {
	for _, v := range *r {
		if v == rec {
			return true
		}
	}
	return false
}

// NewLogger allocates and returns new logger.
func NewLogger() *Logger {
	l := new(Logger)
	l.recorders = make(map[RecorderID]RecorderInterface)
	l.recordersInit = make(map[RecorderID]bool)
	l.severityMasks = make(map[RecorderID]MsgFlagT)
	l.severityOrder = make(map[RecorderID]*list.List)
	return l
}

func (L *Logger) NumberOfRecorders() int {
	L.RLock()
	defer L.RUnlock()
	return len(L.recorders)
}

// RegisterRecorder registers the recorder in the logger with the given id.
// This function receives optional parameter 'asDefault', which says whether
// the need to set it as default recorder. If the optional parameter is not
// specified, it will have a true value.
func (L *Logger) RegisterRecorder(
	id RecorderID,
	intrf RecorderInterface,
	asDefault ...bool,
) error {

	if CfgGlobalDisable.Get() {
		return nil
	}
	if id == RecorderID("") {
		return ErrWrongParameter
	}
	if intrf.ChCtl == nil || intrf.ChMsg == nil {
		return ErrWrongParameter
	}

	if len(asDefault) == 0 {
		asDefault = append(asDefault, true)
	}

	L.Lock()
	defer L.Unlock()

	if L.recorders == nil {
		// We should provide Logger as exported type. So, these checks
		// are necessary because the object can be created w/o New Logger()
		L.recorders = make(map[RecorderID]RecorderInterface)
	} else {
		for recID, _ := range L.recorders {
			if recID == id {
				return ErrWrongRecorderID
			}
		}
	}
	L.recorders[id] = intrf

	// setup initialisation state
	if L.recordersInit == nil {
		L.recordersInit = make(map[RecorderID]bool)
	}
	L.recordersInit[id] = false

	// setup default severity mask
	if L.severityMasks == nil {
		L.severityMasks = make(map[RecorderID]MsgFlagT)
	}
	L.severityMasks[id] = SeverityAll // working with all severities by default

	/* UNREACHABLE
	// check defaults for duplicates
	for i, recID := range L.defaults {
		if recID == id {
			// if id not found in recorders, defaults can't contain it
			L.defaults[i] = L.defaults[len(L.defaults)-1]
			L.defaults[len(L.defaults)-1] = RecorderID("")
			L.defaults = L.defaults[:len(L.defaults)-1]
		}
	}
	*/

	// set as default if necessary
	if asDefault[0] {
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

// UnregisterRecorder disconnects specified recorder from the logger
// (sends a close signal) and removes recorder interface from the logger.
func (L *Logger) UnregisterRecorder(id RecorderID) error {
	if CfgGlobalDisable.Get() {
		return nil
	}
	if id == RecorderID("") {
		return ErrWrongParameter
	}

	L.RLock()

	if len(L.recorders) == 0 {
		L.RUnlock()
		return ErrNoRecorders
	}

	// close recorder if necessary
	if rc, exist := L.recorders[id]; !exist {
		L.RUnlock()
		return ErrWrongRecorderID
	} else {
		if L.recordersInit != nil {
			if initialised, exist := L.recordersInit[id]; !exist {
				// UNREACHABLE //
				L.RUnlock()
				return internalCritical("xlog: missing valid id (.recordersInit)") // PANIC
			} else {
				if initialised {
					if rc.ChCtl == nil {
						L.RUnlock()
						return internalCritical("xlog: sending to nil channel") // PANIC
					}
					rc.ChCtl <- SignalClose()
				}
			}
		} else {
			L.RUnlock()
			return internalError(errMsgBumpedToNil)
		}
	}

	L.RUnlock()
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
	delete(L.recordersInit, id)
	delete(L.severityMasks, id)
	delete(L.severityOrder, id)

	L.Unlock()
	return nil
}

// Initialise sends an initialisation signal to each registered recorder.
func (L *Logger) Initialise(objects ...ListOfRecorders) error {
	if CfgGlobalDisable.Get() {
		return nil
	}

	L.Lock()
	defer L.Unlock()

	if L.initialised {
		return nil
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}
	if L.severityMasks == nil || L.severityOrder == nil {
		return internalError(errMsgBumpedToNil)
	}

	br := BatchResult{}
	br.SetMsg("some of the given recorders are not initialised")
main_cycle:
	for id, rec := range L.recorders {
		if initialised, exist := L.recordersInit[id]; exist {
			if initialised {
				continue
			}
			if len(objects) != 0 {
				for _, list := range objects {
					recorderObject := list.FindByID(rec.id)
					if recorderObject != nil {
						if !recorderObject.IsListening() {
							if CfgAutoStartListening.Get() {
								go recorderObject.Listen()
								runtime.Gosched()
								break
							} else {
								br.Fail(id, ErrNotListening)
								continue main_cycle
							}
						}
					}
				}
			}
			chErr := make(chan error)
			rec.ChCtl <- SignalInit(chErr)
			err := <-chErr
			if err != nil {
				br.Fail(id, err)
			} else {
				if L._falseInit.check(id) {
					// DEBUG, used for tests
					br.Fail(id, _ErrFalseInit)
				} else { // REGULAR
					L.recordersInit[id] = true
					br.OK(id)
				}
			}
		} else {
			// UNREACHABLE //

			// recorder is registered but id is missing in states map
			return internalCritical("xlog: missing valid id (.recordersInit)") // PANIC
		}
	}

	//=== LOGIC AT PARTIAL INITIALISATION ===//
	if br.GetErrors() != nil {
		// all recorders should be initialised for success state
		return br
	} else {
		L.initialised = true
		return nil
	}
}

// Close disconnects (sends a close signal) all registered recorders
// and sets the 'uninitialised' state for the logger. Meanwhile, it
// does not unregister (remove from the logger) recorders.
func (L *Logger) Close() {
	L.Lock()
	defer L.Unlock()

	if !L.initialised {
		return
	}
	if len(L.recorders) == 0 {
		return
	}
	for _, rec := range L.recorders {
		rec.ChCtl <- SignalClose()
	}

	L.initialised = false
}

// DefaultsSet sets given recorders as default for this logger.
func (L *Logger) DefaultsSet(recorders []RecorderID) error {
	if CfgGlobalDisable.Get() {
		return nil
	}

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
		} else {
			br.OK(recID)
		}
	}

	L.defaults = recorders

	if br.GetErrors() != nil {
		return br
	}
	return nil
}

// DefaultsAdd adds given recorders into the default list of the logger.
func (L *Logger) DefaultsAdd(recorders []RecorderID) error {
	if CfgGlobalDisable.Get() {
		return nil
	}

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
		// set as default
		L.defaults = append(L.defaults, recID)
		br.OK(recID)
	}

	if br.GetErrors() != nil {
		return br
	}
	return nil
}

// DefaultsRemove removes given recorders form defaults in this logger.
func (L *Logger) DefaultsRemove(recorders []RecorderID) error {
	if CfgGlobalDisable.Get() {
		return nil
	}

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

	if br.GetErrors() != nil {
		return br
	}
	return nil
}

// ChangeSeverityOrder changes severity order for the specified recorder.
// It takes specified flag and moves it before/after the target flag position.
//
// Only custom flags can be moved (currently disabled).
func (L *Logger) ChangeSeverityOrder(
	recorder RecorderID, srcFlag MsgFlagT, dir ssDirection, trgFlag MsgFlagT,
) error {

	if CfgGlobalDisable.Get() {
		return nil
	}
	if recorder == RecorderID("") {
		return ErrWrongParameter
	}

	L.RLock()

	if len(L.recorders) == 0 {
		L.RUnlock()
		return ErrNoRecorders
	}
	if _, exist := L.recorders[recorder]; !exist {
		L.RUnlock()
		return ErrWrongRecorderID
	}
	if L.severityOrder == nil {
		L.RUnlock()
		return internalError(errMsgBumpedToNil)
	}

	// DISABLED CURRENTLY (TODO)
	//srcFlag = srcFlag &^ 0xCFFF // only custom flags are moveable
	srcFlag = srcFlag &^ SeverityShadowMask
	trgFlag = trgFlag &^ SeverityShadowMask
	if srcFlag == 0 {
		L.RUnlock()
		return ErrWrongFlagValue
	}
	if trgFlag == 0 {
		L.RUnlock()
		return ErrWrongFlagValue
	}

	if srcFlag == trgFlag {
		L.RUnlock()
		return ErrWrongFlagValue
	}

	orderlist, exist := L.severityOrder[recorder]
	if !exist { // UNREACHABLE //
		L.RUnlock()
		return internalCritical("xlog: missing valid id (.severityOrder)") // PANIC
	}
	var src *list.Element
	var trg *list.Element
	for e := orderlist.Front(); e != nil; e = e.Next() {
		if sev, ok := e.Value.(MsgFlagT); !ok {
			L.RUnlock()
			return internalCritical("xlog: unexpected type") // PANIC
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
		L.RUnlock()
		//return internalError("can't find flag <%012b> (.severityOrder)", srcFlag)
		return internalCritical("xlog: can't find flag (.seveirtyOrder)") // PANIC
	}
	if trg == nil {
		L.RUnlock()
		//return internalError("can't find trg flag <%012b> (.severityOrder)", trgFlag)
		return internalCritical("xlog: can't find flag (.severityOrder)") // PANIC
	}

	L.RUnlock()
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
	if CfgGlobalDisable.Get() {
		return nil
	}
	if recorder == RecorderID("") {
		return ErrWrongParameter
	}

	L.Lock()
	defer L.Unlock()

	if L.severityMasks == nil {
		return internalError(errMsgBumpedToNil)
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}

	if sevMask, exist := L.severityMasks[recorder]; !exist {
		// already failed in this case, we should choose error here
		if _, exist := L.recorders[recorder]; !exist {
			return ErrWrongRecorderID
		} else {
			// UNREACHABLE //
			return internalCritical("xlog: missing valid id (.severityMasks)") // PANIC
		}
		_ = sevMask // THAT'S COMPLETELY STUPID, GOLANG
	} else {
		// zero is allowed (recorder blocked) //
		L.severityMasks[recorder] = flags &^ SeverityShadowMask
	}

	return nil
}

// Write builds the message with format line and specified message flags, then calls
// WriteMsg. It allows avoiding calling fmt.Sprintf() function and LogMsg's functions
// directly, it wraps all of it.
//
// Returns nil in case of success otherwise returns an error.
func (L *Logger) Write(flags MsgFlagT, msgFmt string, msgArgs ...interface{}) error {
	if CfgGlobalDisable.Get() {
		return nil
	}
	msg := NewLogMsg().SetFlags(flags)
	msg.Setf(msgFmt, msgArgs...)
	return L.WriteMsg(nil, msg)
}

// WriteMsg send write signal with given message to the specified recorders.
// If custom recorders are not specified, uses default recorders of this logger.
//
// Returns nil on success and error on fail.
func (L *Logger) WriteMsg(recorders []RecorderID, msg *LogMsg) error {
	// {Logger}: only read access

	if CfgGlobalDisable.Get() {
		return nil
	}
	if msg == nil {
		return ErrWrongParameter
	}

	L.RLock()
	defer L.RUnlock()

	if !L.initialised {
		return ErrNotInitialised
	}
	if len(L.recorders) == 0 {
		return ErrNoRecorders
	}
	if L.severityMasks == nil || L.severityOrder == nil {
		return internalError(errMsgBumpedToNil)
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
			// RecorderID("") is not possible //
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
			// UNREACHABLE //
			//br.Fail(recID, internalError(".severityMasks -> missing valid id (unreachable)"))
			return internalCritical("xlog: missing valid id (.severityMasks)") // PANIC
		}
	}

	// write errors ain't possible currently
	//if br.GetErrors() != nil { return br }
	return nil
}

// This function actually has got a protector role because in some places
// a severity argument should have only one of these flags. So it ensures
// (accordingly to the depth order) that severity value provide only one
// flag.
func (L *Logger) severityProtector(orderlist *list.List, flags *MsgFlagT) error {
	if orderlist == nil || orderlist.Len() == 0 {
		return internalError("[severityProtector] wrong 'orderlist' parameter value")
	}
	for e := orderlist.Front(); e != nil; e = e.Next() {
		if sev, ok := e.Value.(MsgFlagT); ok {
			if (*flags&^SeverityShadowMask)&sev > 0 {
				*flags = *flags &^ (^SeverityShadowMask) // reset
				*flags = *flags | sev                    // set
				return nil
			}
		} else {
			return internalCritical("xlog: unexpected type") // PANIC
		}
	}
	//return internalError("can't find severity flag in orderlist <%012b>", *flags)
	return internalCritical("xlog: can't find severity flag (orderlist)") // PANIC
}
