## General

`xlog` is a library for easily logging into multiple endpoints, such as files,
stdout, stderr, syslog, etc at the same time. Configure logger(s) once, and then
write to several log endpoints according to defined rules with one function call.
Flexible configuration of loggers allows controlling which recorders will be used
in different cases.

## Overview

`LogRecorder` objects represent log endpoints and provide write function and other
methods to correctly interact with them. Recorders connect to one or more `Logger`
objects which control them and determinate which log messages they should write and
when. Basically all what recorder do it is listening signals from connected loggers
and write the specific log messages when WRITE signal received.

`Logger` objects unite several recorders to correspond to some context. You can
specify bitmasks for each connected recorder to control which recorders will be used
for each severity (log level). Every logger also has a list of default recorders
which will be used by default for writing if a custom list is not specified at the
write function call.

## Usage

> 🛈 Use godoc to get complete package documentation.

For example, we want to write logs to the *stdout* and 2 log files. Files should
be used as primary endpoints: one for the info messages and one for the errors;
*stdout* gonna be optional endpoint (will use it manually when we need).

First, you should create `LogRecorder` objects for necessary endpoints. Usually,
you will need one recorder per endpoint. Also, we need `Logger` object to control
recorders. If you have several contexts in your app, you probably may need several
loggers.

Create and activate recorders:
```go
// ...

stdoutRecorder  := xlog.NewIoDirectRecorder(os.Stdout)
infFileRecorder := xlog.NewIoDirectRecorder(hFileInfo)
errFileRecorder := xlog.NewIoDirectRecorder(hFileError)

go stdoutRecorder.Listen()
go infFileRecorder.Listen()
go errFileRecorder.Listen()

// defer func() { stdoutRecorder.Intrf().ChCtl  <- xlog.SignalStop() }()
// defer func() { intFileRecorder.Intrf().ChCtl <- xlog.SignalStop() }()
// defer func() { errFileRecorder.Intrf().ChCtl <- xlog.SignalStop() }()
```
OR you can use the `SpawnXXX` functions (that's the recommended way). 
```go
stdoutRecorder  := xlog.SpawnIoDirectRecorder(os.Stdout)
infFileRecorder := xlog.SpawnIoDirectRecorder(hFileInfo)
errFileRecorder := xlog.SpawnIoDirectRecorder(hFileError)
```

Declare recorder IDs which will be used in the logger to access the recorders.
```go
recStdout := xlog.RecorderID("rec-stdout")
recInfo   := xlog.RecorderID("rec-finfo")
recErr    := xlog.RecorderID("rec-ferr")
```

Create `Logger` and connect recorders:
```go
logger := xlog.NewLogger()
_ = logger.RegisterRecorder(recStdout, stdoutRecorder.Intrf())
_ = logger.RegisterRecorder(recInfo,  infFileRecorder.Intrf())
_ = logger.RegisterRecorder(recErr,   errFileRecorder.Intrf())
```
`Logger.RegisterRecorder()` registers recorder in the logger and bind it to the
given ID. Logger can interact only with registered recorders.

Configure the logger:
```go
_ = logger.DefaultsSet([]RecorderID{recInfo, recErr})
_ = logger.SetSeverityMask(recInfo, xlog.SeverityMinor | xlog.Warning) // Notice | Info | Warning
_ = logger.SetSeverityMask(recErr,  xlog.SeverityMajor) // Warning | Emerg | Alert | Critical | Error
```
Here we set *infFileRecorder* and *errFileRecorder* as default recorders for the logger.
These recorders will be used by default unless otherwise specified. So, to write into the
*stdoutRecorder*, we need manually pass *recStdout* id at the `Logger.WriteMsg()` call.
Also, we set severity masks for the recorders: *errFileRecorder* will write only "bad"
severities ignoring messages with `Notice`, `Info` and `Debug` flags; *infFileRecorder*
opposite will write only `Info`, `Notice` and `Warning` (optional) messages. This allows
the recorder to automatically distribute messages to the correct recorders, you need to
specify only severity flag for the message.

And finally, initialise it:
```go
if err := logger.Initialise(); err != nil {
    os.Exit(1)
} else {
    defer func() { // at exit
        logger.Close()
    }()
}
```

To log something just call `Logger.Write()`. This function receives severity and attribute
flags as first parameter and message with arguments as second and further (like fmt.Printf).
`Logger.Write()` always use default recorders of the logger.
```go
logger.Write(xlog.Info, "my message %d", 123)     // will be send to the infFileRecorder
logger.Write(xlog.Error, "something went wrong")  // will be send to the errFileRecorder
logger.Write(xlog.Warning, "imp. notification")   // will be send to both file recorders
logger.Write(xlog.Debug, "some debug info")       // this message will be ignored
```

If you have a long operation and you want to handle it with a single log message,
you can use `LogMsg` to construct a complex message. In this case, you should use
`Logger.WriteMsg()` instead.
```go
msg := NewLogMsg().
    SetFlags(xlog.Info).
    Setf("message header\n")
// ... do something
msg.Addf("  op1: OK\n")
// .. do something
msg.Addf("  op2: OK")
msg.UpdateTime()

logger.WriteMsg(nil, msg)
```

To manually select which recorders (of a specific logger) should be used to handle
a message, you should use `Logger.WriteMsg()` with specified list of recorders IDs
as the first argument. Otherwise (nil argument), logger will use default recorders.
```go
// write only to the stdout recorder (registered earlier as recStdout)
logger.WriteMsg([]RecorderID{recStdout}, xlog.Message("my message %d", 123))

// send Critical message to all recorders
recAll := []RecorderID{recStdout, recInfo, recErr}
logger.WriteMsg(recAll, xlog.Message("my message").SetFlags(xlog.Critical))
```
*`xlog.Message("msg")` is equivalent to `xlog.NewMsg().Setf("msg")`*

### Some advanced features

#### Safe initialisation

`Logger.Initialise()` can receive an optional parameter - list of recorder
objects. If it specified the function will use it to call `LogRecorder.IsListening()`
functions to ensure that recorders are ready to receive the signals. Because if some
of the recorders are not listening, initialisation call may lock the goroutine. Besides
if parameter `xlog.cfgAutoStartListening` is enabled, the function can call a listener
by self and continue without an error.

So, if you don't use `SpawnXXX` functions, this way is recommended:
```go
l := xlog.NewLogger()
var gRecorders xlog.ListOfRecorders
r1 := xlog.NewIoDirectRecorder(os.Stdout)
r2 := xlog.NewSyslogRecorder("my-prefix")
go r1.Listen()
go r2.Listen()
gRecorders.Add(r1)
gRecorders.Add(r2)

l.RegisterRecorder("REC-1", r1.Intrf())
l.RegisterRecorder("REC-2", r2.Intrf())

if err := l.Initialise(gRecorders); err != nil {
    if err == xlog.ErrNotListening { ... }
}
```

#### Handling write errors

Default recorders just skip write signal in case of error to do not lock a caller
goroutine. It means that you will not be notified if the error happens. So, if you
need to handle msg write errors, you can attach external channel to receive error
messages (it's errors from the internal write function only).

First, you need to create a receiver: it must be an async channel `chan error` with a
sufficient buffer size so as not to block the recorder. You should provide the proper
usage of this channel(s). If you use a single handler for all your recorders, you have
to make sure that it will not be closed or locked while recorders use it.

To set a channel as an error handler for the default recorder you need to send
`SigSetErrChan` control signal. To construct this signal use `SignalSetErrChan`
function. To drop the channel for the logger send `SigDropErrChan` signal  using
`SignalDropErrChan` function.

```go
chErr := make(chan error, 256)
go func() {
    for msg := range chErr {
        if msg == nil { continue } // unreachable
        fmt.Printf("RECORDER ERROR: %s\n", msg.Error())
    }
}

r := xlog.NewIoDirectRecorder(os.Stdout, "my-prefix")
r.Intrf().ChCtl <- SignalSetErrChan(chErr)
runtime.Gosched()

// ...

r.Intrf().ChCtl <- SignalDropErrChan()
time.Sleep(time.Second)
close(chErr)
```

-----

**...**

### Log message formatters & custom flags

You can control the recorder's output by custom format functions. Recorder will call
a formatter before the writing to construct a final output string. This functions
should implement the interface: `type FormatFunc func(*xlog.LogMsg) string`. To set
format function call `LogRecorder.FormatFunc()`.

For example, to get colored output, you can do this:
```go
r := xlog.NewIoDirectRecorder(os.Stdout).FormatFunc( func(msg *xlog.LogMsg) string {
    // drop attributes, get severity flags only
    sev := msg.GetFlags() &^ xlog.SeverityShadowMask

    // Logger.WriteMsg ensures that several severity flags
    // issue is not possible here; we can use switch here
    switch (sev) {
    case xlog.Emerg:
        return fmt.Sprintf("\x1b[30;41m%s\x1b[0m", msg.GetContent())
    case xlog.Alert:
        return fmt.Sprintf("\x1b[30;41m%s\x1b[0m", msg.GetContent())
    case xlog.Critical:
        return fmt.Sprintf("\x1b[30;41m%s\x1b[0m", msg.GetContent())
    case xlog.Error:
        return fmt.Sprintf("\x1b[31m%s\x1b[0m", msg.GetContent())
    case xlog.Warning:
        return fmt.Sprintf("\x1b[33m%s\x1b[0m", msg.GetContent())
    case xlog.Notice:
        return fmt.Sprintf("\x1b[1m%s\x1b[0m", msg.GetContent())
    default:
        return msg.GetContent()
    }
})
```

Furthermore, `LogMsg` has *Data* field which can be used to pass any kind additional
information into the formatter:
```go
type payload struct {
  a string
  b int
}

func formatter(msg *xlog.LogMsg) string {
    var extra string
    if data, ok := msg.Data.(payload); ok {
        extra = fmt.Sprintf(" with %v", data)
    }
    return msg.GetContent() + extra
}

// ...

msg := NewLogMsg().Setf("message")
msg.Data = payload{"extra", 83485}
logger.WriteMsg(nil, msg)
```

Besides 10 default flags (8 severities and 2 attributes)
custom flags are available. You can declare em like this:
```go
var MySeverity1 xlog.MsgFlagT = xlog.CustomB1
var MySeverity1 xlog.MsgFlagT = xlog.CustomB2

var MyAttribute1 xlog.MsgFlagT = xlog.CustomB3
var MyAttribute2 xlog.MsgFlagT = xlog.CustomB4
```

With custom flags and message formatters you can realise
extra functionality without creating a custom recorder.

## Custom recorders

> TL;DW  
> Docs coming soon. For now, you can use `rec_direct.go` as an example.
