package xlog

// This tool can be used to control recorders and logger state and debug concurrency
// and synchronisation errors. Pass channel returned by debugLogger.Chan() function
// to the recorder as debug channel to collect debugging data.

// Built-in recorders will write to debug channel automatically if it passed.
// You can also write user messages to it: d.Chan() <- DbgMsg("my message")
// To stop listener just close the channel: close(d.Chan())

// BE CAREFUL WITH CHANNEL CLOSING: it can be closed only after the all ops.
// Another way you can call recorder.DropDebugger() for all recorders to disconnect
// debug channel which allows the recorder to continue normal work without debugging.

import (
	"fmt"
	"io"
	"time"
)

type debugMessage struct {
	time time.Time
	data string
}

func DbgMsg(format string, args ...interface{}) debugMessage {
	return debugMessage{
		data: fmt.Sprintf(format, args...),
		time: time.Now(),
	}
}

type debugLogger struct {
	listening bool
	channel   chan debugMessage
	writer    io.Writer
}

func NewDebugLogger(writer io.Writer) *debugLogger {
	if writer == nil {
		return nil
	}
	d := new(debugLogger)
	d.writer = writer
	d.channel = make(chan debugMessage, 10)
	return d
}

func (D *debugLogger) Listen() {
	if D.listening || D.writer == nil {
		return
	}
	D.listening = true
	fmt.Print(".....XLOG DEBUG LISTENER STARTED.....\n")
	D.writer.Write([]byte(fmt.Sprintf("-------------------- %v\n", time.Now())))

	for msg := range D.channel {
		msg := fmt.Sprintf("[%s] %s", msg.time.Format("15:04:05.000000000"), msg.data)
		if msg[len(msg)-1] != '\n' {
			msg += "\n"
		}
		if _, err := D.writer.Write([]byte(msg)); err != nil {
			fmt.Printf(".....XLOG DEBUG LISTENER WRITE ERROR.....\n  %v\n", err)
		}
	}

	D.listening = false
	fmt.Print(".....XLOG DEBUG LISTENER STOPPED.....\n")
	fmt.Printf("event time: %s\n", time.Now().Format("15:04:05.000000000"))
}

func (D *debugLogger) Chan() chan<- debugMessage {
	return D.channel
}
