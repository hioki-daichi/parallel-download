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

// CleanFunc registers clean function.
func CleanFunc(f func()) {
	cleanFns = append(cleanFns, f)
}

// Listen listens signals.
func Listen(w io.Writer) {
	listen(w, func() { os.Exit(0) })
}

func listen(w io.Writer, exitFn func()) {
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
