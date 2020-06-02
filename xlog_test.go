package xlog

import (
	"fmt"
	"os"
	"testing"
	"time"
)

var dc chan<- debugMessage

func TestMain(m *testing.M) {
	dbgFile, err := os.OpenFile("dbg.outp", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("dbg file open fail: %s", err.Error())
		os.Exit(1)
	}
	d := NewDebugLogger(dbgFile)
	dc = d.Chan() // yeah, it can drops
	go d.Listen()

	r := m.Run()

	time.Sleep(time.Millisecond)
	//d.Close()
	dbgFile.Close()
	os.Exit(r)
}
