package xlog

// This tool can be used to control recorders and logger state and debug concurrency
// and synchronisation errors. Pass channel returned by debugLogger.Chan() function
// to the recorder as debug channel to collect debugging data.

// Built-in recorders will write to debug channel automatically if it passed. You
// can also write user messages to it: d.Chan() <- DbgMsg(xid.NilID(), "my message")
// To stop debug listener just close the channel: close(d.Chan())

// BE CAREFUL WITH CHANNEL CLOSING: it can be closed only after all ops ended.
// Another way you can send SigDropDbgChan for all recorders to disconnect
// debug channel which allows the recorder to continue normal work without
// debugging.

import (
	"fmt"
	"io"
	"time"

	"github.com/rs/xid"
)

type debugMessage struct {
	rtype string // recorder type (sets by caller manually)
	id    xid.ID
	time  time.Time
	data  string
}

func DbgMsg(recorderXID xid.ID, format string, args ...interface{}) debugMessage {
	return debugMessage{rtype: "?",
		id:   recorderXID,
		data: fmt.Sprintf(format, args...),
		time: time.Now(),
	}
}

type debugLogger struct {
	//sync.RWMutex
	isListening bool_s
	channel     chan debugMessage
	writer      io.Writer
}

func NewDebugLogger(writer io.Writer) *debugLogger {
	if writer == nil {
		return nil
	}
	d := new(debugLogger)
	d.channel = make(chan debugMessage, 64)
	d.writer = writer
	return d
}

func (D *debugLogger) Close() {
	close(D.channel) // ATTENTION
	D.channel = nil
}

func (D *debugLogger) Listen() {
	if D.isListening.Get() || D.writer == nil {
		return
	}
	D.isListening.Set(true)
	fmt.Print(".....XLOG DEBUG LISTENER STARTED.....\n")
	D.writer.Write([]byte(fmt.Sprintf("-------------------- %v\n", time.Now())))

	for msg := range D.channel {
		msg := fmt.Sprintf("[%s] {%s:%s} %s",
			msg.time.Format("15:04:05.000000000"), msg.rtype, msg.id.String(), msg.data)
		if msg[len(msg)-1] != '\n' {
			msg += "\n"
		}
		if _, err := D.writer.Write([]byte(msg)); err != nil {
			fmt.Printf(".....XLOG DEBUG LISTENER WRITE ERROR.....\nevent time: %s\nerror: %v\n",
				time.Now().Format("15:04:05.000000000"), err)
		}
	}

	D.isListening.Set(false)
	fmt.Print(".....XLOG DEBUG LISTENER STOPPED.....\n")
	fmt.Printf("event time: %s\n", time.Now().Format("15:04:05.000000000"))
}

func (D *debugLogger) Chan() chan<- debugMessage {
	return D.channel
}
