package xlog

import (
	"fmt"
	"time"
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

// NewlogMsg allocates and returns a new logMsg. 
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
