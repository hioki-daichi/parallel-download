package downloader

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/hioki-daichi/parallel-download/bytesranger"
)

var errExist = errors.New("downloader: file already exists")

// Downloader has URL.
type Downloader struct {
	// Usually, stdout is specified, and at the time of testing, buffer is specified.
	OutStream io.Writer

	// URL represents the download URL.
	URL string

	// Parallelism represents how many parallel downloads.
	Parallelism int
}

// NewDownloader returns new Downloader.
func NewDownloader(w io.Writer, url string) *Downloader {
	return &Downloader{OutStream: w, URL: url, Parallelism: 4} // TODO: Use flags instead of hard-coded 4
}

// Download downloads the file with the specified URL.
func (d *Downloader) Download() error {
	_, filename := path.Split(d.URL)
	if isFileExist(filename) {
		return errExist
	}

	bytesrangeStrings, err := d.getBytesrangeStrings()
	if err != nil {
		return err
	}

	ch := make(chan map[int]*http.Response)

	// send to channels...
	for i, bytesrangeString := range bytesrangeStrings {
		i := i
		bytesrangeString := bytesrangeString
		go func() {
			resp, err := d.getHTTPResponseWithinRange(bytesrangeString)
			if err != nil {
				panic(err) // TODO: error handling
			}

			fmt.Fprintf(d.OutStream, "ch snd [i: %d, ContentLength: %d, Range: %s]\n", i, resp.ContentLength, bytesrangeString)

			ch <- map[int]*http.Response{i: resp}
		}()
	}

	// receive channels...
	responses := make(map[int]*http.Response, 0)
	for i := 0; i < len(bytesrangeStrings); i++ {
		m := <-ch

		for i, resp := range m {
			fmt.Fprintf(d.OutStream, "ch rcv [i: %d, ContentLength: %d]\n", i, resp.ContentLength)

			responses[i] = resp
		}
	}

	fp, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// concat responses...
	for i := 0; i < len(responses); i++ {
		resp := responses[i]
		_, err := io.Copy(fp, resp.Body)
		if err != nil {
			os.Remove(filename)
			return err
		}
	}

	fmt.Fprintf(d.OutStream, "Downloaded: %q\n", d.URL)

	return nil
}

func (d *Downloader) getBytesrangeStrings() ([]string, error) {
	resp, err := http.Head(d.URL)
	if err != nil {
		return nil, err
	}

	return bytesranger.Split(int(resp.ContentLength), d.Parallelism)
}

func (d *Downloader) getHTTPResponseWithinRange(rangeString string) (*http.Response, error) {
	client := &http.Client{Timeout: 0}

	req, err := http.NewRequest("GET", d.URL, nil)
	if err != nil {
		return &http.Response{}, err
	}

	req.Header.Set("Range", rangeString)

	return client.Do(req)
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
