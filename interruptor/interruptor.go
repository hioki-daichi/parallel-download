/*
Package interruptor deals with Ctrl+C interruption.
*/
package interruptor

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

var cleanFns []func()

// RegisterCleanFunction registers clean function.
func RegisterCleanFunction(f func()) {
	cleanFns = append(cleanFns, f)
}

// Setup setups interrupt handler.
func Setup(w io.Writer) {
	setup(w, func() { os.Exit(0) })
}

func setup(w io.Writer, exitFn func()) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		fmt.Fprintln(w, "\rCtrl+C pressed in Terminal")
		for _, f := range cleanFns {
			f()
		}
		exitFn()
	}()
}
