package interruptor

import (
	"os"
	"testing"
)

func TestInterruptor_RegisterCleanFunction(t *testing.T) {
	if len(cleanFns) != 0 {
		t.Fatal("Unexpectedly cleanFns has already been set")
	}
	RegisterCleanFunction(func() {})
	if len(cleanFns) != 1 {
		t.Errorf(`unexpected cleanFns length: expected: 1 actual: %d`, len(cleanFns))
	}
}

func TestInterruptor_setup(t *testing.T) {
	cleanFns = append(cleanFns, func() {})
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("err %s", err)
	}
	doneCh := make(chan struct{})
	setup(func() { doneCh <- struct{}{} })
	proc.Signal(os.Interrupt)
	<-doneCh
}
