package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/hioki-daichi/parallel-download/downloading"
	"github.com/hioki-daichi/parallel-download/interruptor"
	"github.com/hioki-daichi/parallel-download/opt"
)

func main() {
	err := execute(os.Stdout, os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}

func execute(w io.Writer, args []string) error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interruptor.Setup(w)
	interruptor.RegisterCleanFunction(cancel)

	opts, err := opt.Parse(args...)
	if err != nil {
		return err
	}

	return downloading.NewDownloader(w, opts).Download(ctx)
}
