/*
Package terminator deals with Ctrl+C termination.
*/
package terminator

import (
	"context"
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
func Listen(ctx context.Context, w io.Writer) (context.Context, func()) {
	return listen(ctx, w, func() { os.Exit(0) })
}

func listen(ctx context.Context, w io.Writer, exitFn func()) (context.Context, func()) {
	ctx, cancel := context.WithCancel(ctx)

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		fmt.Fprintln(w, "\rCtrl+C pressed in Terminal")
		cancel()
		for _, f := range cleanFns {
			f()
		}
		exitFn()
	}()

	return ctx, cancel
}
