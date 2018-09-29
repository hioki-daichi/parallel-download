package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/hioki-daichi/parallel-download/downloading"
	"github.com/hioki-daichi/parallel-download/opt"
	"github.com/hioki-daichi/parallel-download/terminator"
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

	terminator.Listen(w)
	terminator.CleanFunc(cancel)

	opts, err := opt.Parse(args...)
	if err != nil {
		return err
	}

	return downloading.NewDownloader(w, opts).Download(ctx)
}
