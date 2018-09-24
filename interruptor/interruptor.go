/*
Package interruptor deals with Ctrl+C interruption.
*/
package interruptor

import (
	"fmt"
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
func Setup() {
	setup(func() { os.Exit(0) })
}

func setup(exitFn func()) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		for _, f := range cleanFns {
			f()
		}
		exitFn()
	}()
}
