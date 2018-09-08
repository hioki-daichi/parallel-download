package downloader

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
)

var errExist = errors.New("downloader: file already exists")

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
	_, filename := path.Split(d.URL)
	if isFileExist(filename) {
		return errExist
	}

	resp, err := http.Get(d.URL)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fp, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	_, err = fp.Write(body)
	if err != nil {
		return err
	}

	fmt.Fprintf(d.OutStream, "Downloaded: %q\n", d.URL)

	return nil
}

func isFileExist(filename string) bool {
	_, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL, 0644)
	if os.IsExist(err) {
		return true
	}

	// If the file does not exist, delete the garbage file.
	os.Remove(filename)

	// If any other unexpected error occurs, then panic.
	if err != nil {
		panic(err)
	}

	return false
}
