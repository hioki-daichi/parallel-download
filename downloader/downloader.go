package downloader

import (
	"fmt"
	"io"
)

// Downloader has URL.
type Downloader struct {
	// Usually, stdout is specified, and at the time of testing, buffer is specified.
	OutStream io.Writer

	// URL represents the download URL.
	URL string
}

// NewDownloader returns new Downloader.
func NewDownloader(w io.Writer, url string) *Downloader {
	return &Downloader{OutStream: w, URL: url}
}

// Download downloads the file with the specified URL.
func (d *Downloader) Download() error {
	// TODO: implement
	fmt.Fprintf(d.OutStream, "Downloaded: %q\n", d.URL)
	return nil
}
