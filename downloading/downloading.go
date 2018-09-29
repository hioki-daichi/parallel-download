/*
Package downloading provides download function.
*/
package downloading

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hioki-daichi/parallel-download/opt"
	"github.com/hioki-daichi/parallel-download/terminator"
	"golang.org/x/sync/errgroup"
)

var (
	errResponseDoesNotIncludeAcceptRangesHeader = errors.New("response does not include Accept-Ranges header")
	errValueOfAcceptRangesHeaderIsNotBytes      = errors.New("the value of Accept-Ranges header is not bytes")
	errNoContent                                = errors.New("no content")
)

// Downloader has the information for the download.
type Downloader struct {
	outStream   io.Writer
	url         *url.URL
	parallelism int
	output      string
	timeout     time.Duration
}

// NewDownloader generates Downloader based on Options.
func NewDownloader(w io.Writer, opts *opt.Options) *Downloader {
	return &Downloader{
		outStream:   w,
		url:         opts.URL,
		parallelism: opts.Parallelism,
		output:      opts.Output,
		timeout:     opts.Timeout,
	}
}

// Download performs parallel download.
func (d *Downloader) Download(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	contentLength, err := d.getContentLength(ctx)
	if err != nil {
		return err
	}

	rangeHeaders := d.toRangeHeaders(contentLength)

	tempDir, clean, err := createTempDir()
	if err != nil {
		return err
	}
	defer clean()

	filenames, err := d.parallelDownload(ctx, rangeHeaders, tempDir)
	if err != nil {
		return err
	}

	fmt.Fprintln(d.outStream, "create destination tempfile")
	tempFile, err := os.Create(filepath.Join(tempDir, genUUID()))
	if err != nil {
		return err
	}
	fmt.Fprintf(d.outStream, "created: %q\n", tempFile.Name())

	fmt.Fprintln(d.outStream, "concat downloaded files to destination tempfile")
	for i := 0; i < len(filenames); i++ {
		src, err := os.Open(filenames[i])
		if err != nil {
			return err
		}

		_, err = io.Copy(tempFile, src)
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(d.outStream, "rename destination tempfile to %q\n", d.output)
	err = os.Rename(tempFile.Name(), d.output)
	if err != nil {
		return err
	}

	fmt.Fprintf(d.outStream, "completed: %q\n", d.output)

	return nil
}

// getContentLength returns the value of Content-Length received by making a HEAD request.
func (d *Downloader) getContentLength(ctx context.Context) (int, error) {
	fmt.Fprintf(d.outStream, "start HEAD request to get Content-Length\n")

	req, err := http.NewRequest("HEAD", d.url.String(), nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return 0, err
	}

	err = d.validateAcceptRangesHeader(resp)
	if err != nil {
		return 0, err
	}

	contentLength := int(resp.ContentLength)

	fmt.Fprintf(d.outStream, "got: Content-Length: %d\n", contentLength)

	if contentLength < 1 {
		return 0, errNoContent
	}

	return contentLength, nil
}

// validateAcceptRangesHeader validates the following.
// - The presence of an Accept-Ranges header
// - The value of the Accept-Ranges header is "bytes"
func (d *Downloader) validateAcceptRangesHeader(resp *http.Response) error {
	acceptRangesHeader := resp.Header.Get("Accept-Ranges")

	fmt.Fprintf(d.outStream, "got: Accept-Ranges: %s\n", acceptRangesHeader)

	if acceptRangesHeader == "" {
		return errResponseDoesNotIncludeAcceptRangesHeader
	}

	if acceptRangesHeader != "bytes" {
		return errValueOfAcceptRangesHeaderIsNotBytes
	}

	return nil
}

// toRangeHeaders converts the value of Content-Length to the value of Range header.
func (d *Downloader) toRangeHeaders(contentLength int) []string {
	parallelism := d.parallelism

	// 1 <= parallelism <= Content-Length
	if parallelism < 1 {
		parallelism = 1
	}
	if contentLength < parallelism {
		parallelism = contentLength
	}

	unitLength := contentLength / parallelism
	remainingLength := contentLength % parallelism

	rangeHeaders := make([]string, 0)

	cntr := 0
	for n := parallelism; n > 0; n-- {
		min := cntr
		max := cntr + unitLength - 1

		// Add the remaining length to the last chunk
		if n == 1 && remainingLength != 0 {
			max += remainingLength
		}

		rangeHeaders = append(rangeHeaders, fmt.Sprintf("bytes=%d-%d", min, max))

		cntr += unitLength
	}

	return rangeHeaders
}

// parallelDownload downloads in parallel for each specified rangeHeaders and saves it in the specified dir.
func (d *Downloader) parallelDownload(ctx context.Context, rangeHeaders []string, dir string) (map[int]string, error) {
	filenames := map[int]string{}

	filenameCh := make(chan map[int]string)
	errCh := make(chan error)

	for i, rangeHeader := range rangeHeaders {
		go d.downloadChunkFile(ctx, i, rangeHeader, filenameCh, errCh, dir)
	}

	eg, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	for i := 0; i < len(rangeHeaders); i++ {
		eg.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case m := <-filenameCh:
				for k, v := range m {
					mu.Lock()
					filenames[k] = v
					mu.Unlock()
				}
				return nil
			case err := <-errCh:
				return err
			}
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return filenames, nil
}

func (d *Downloader) downloadChunkFile(ctx context.Context, i int, rangeHeader string, ch chan<- map[int]string, errCh chan<- error, dir string) {
	req, err := http.NewRequest("GET", d.url.String(), nil)
	if err != nil {
		errCh <- err
		return
	}

	req.Header.Set("Range", rangeHeader)

	fmt.Fprintf(d.outStream, "start GET request with header: \"Range: %s\"\n", rangeHeader)

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		errCh <- err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		errCh <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		return
	}

	tmp, err := os.Create(path.Join(dir, genUUID()))
	if err != nil {
		errCh <- err
		return
	}

	_, err = io.Copy(tmp, resp.Body)
	if err != nil {
		errCh <- err
		return
	}

	fmt.Fprintf(d.outStream, "downloaded: %q\n", tmp.Name())

	ch <- map[int]string{i: tmp.Name()}

	return
}

func createTempDir() (string, func(), error) {
	dir, err := ioutil.TempDir("", "parallel-download")
	if err != nil {
		return "", nil, err
	}
	clean := func() { os.RemoveAll(dir) }
	terminator.CleanFunc(clean)
	return dir, clean, nil
}

func genUUID() string {
	u, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return u.String()
}
