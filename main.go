package main

import (
	"io"
	"log"
	"os"

	"github.com/hioki-daichi/parallel-download/downloader"
)

func main() {
	url := os.Args[1]
	err := execute(os.Stdout, url)
	if err != nil {
		log.Fatal(err)
	}
}

func execute(w io.Writer, url string) error {
	d := downloader.NewDownloader(w, url)
	return d.Download()
}
