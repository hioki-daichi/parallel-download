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

	"github.com/google/uuid"
	"github.com/hioki-daichi/parallel-download/interruptor"
	"github.com/hioki-daichi/parallel-download/opt"
	"golang.org/x/sync/errgroup"
)

var (
	errAcceptRangesHeaderNotFound          = errors.New("Accept-Ranges header not found")
	errAcceptRangesHeaderSupportsBytesOnly = errors.New("Accept-Ranges header supports bytes only")
	errNoContent                           = errors.New("no content")
)

// Downloader has the information for the download.
type Downloader struct {
	outStream   io.Writer
	url         *url.URL
	parallelism int
	output      string
}

// NewDownloader generates Downloader based on Options.
func NewDownloader(w io.Writer, opts *opt.Options) *Downloader {
	return &Downloader{
		outStream:   w,
		url:         opts.URL,
		parallelism: opts.Parallelism,
		output:      opts.Output,
	}
}

// Download performs parallel download.
func (d *Downloader) Download(ctx context.Context) error {
	contentLength, err := d.validateHeaderAndGetContentLength()
	if err != nil {
		return err
	}

	formattedRangeHeaders := d.genFormattedRangeHeaders(contentLength)

	tempDir, cleanFn, err := createTempDir()
	if err != nil {
		return err
	}
	defer cleanFn()

	filenameCh := make(chan map[int]string)
	errCh := make(chan error)
	for i, frh := range formattedRangeHeaders {
		go d.downloadChunkFile(ctx, i, frh, filenameCh, errCh, tempDir)
	}

	filenames := map[int]string{}

	eg, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	for i := 0; i < len(formattedRangeHeaders); i++ {
		eg.Go(func() error {
			select {
			case <-ctx.Done():
				return nil
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

func (d *Downloader) validateHeaderAndGetContentLength() (int, error) {
	fmt.Fprintf(d.outStream, "start HEAD request to get Content-Length\n")

	req, err := http.NewRequest("HEAD", d.url.String(), nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}

	acceptRangesHeader := resp.Header.Get("Accept-Ranges")
	fmt.Fprintf(d.outStream, "got: Accept-Ranges: %s\n", acceptRangesHeader)
	if acceptRangesHeader == "" {
		return 0, errAcceptRangesHeaderNotFound
	}

	if acceptRangesHeader != "bytes" {
		return 0, errAcceptRangesHeaderSupportsBytesOnly
	}

	contentLength := int(resp.ContentLength)

	fmt.Fprintf(d.outStream, "got: Content-Length: %d\n", contentLength)

	if contentLength < 1 {
		return 0, errNoContent
	}

	return contentLength, nil
}

func (d *Downloader) genFormattedRangeHeaders(contentLength int) []string {
	parallelism := d.parallelism

	if parallelism < 1 {
		parallelism = 1
	}

	if contentLength < parallelism {
		parallelism = contentLength
	}

	chunkContentLength := contentLength / parallelism
	rem := contentLength % parallelism

	ss := make([]string, 0)

	cntr := 0
	for n := parallelism; n > 0; n-- {
		min := cntr
		max := cntr + chunkContentLength - 1

		if n == 1 && rem != 0 {
			max += rem
		}

		ss = append(ss, fmt.Sprintf("bytes=%d-%d", min, max))

		cntr += chunkContentLength
	}

	return ss
}

func (d *Downloader) downloadChunkFile(ctx context.Context, i int, formattedRangeHeader string, ch chan<- map[int]string, errCh chan<- error, dir string) {
	req, err := http.NewRequest("GET", d.url.String(), nil)
	if err != nil {
		errCh <- err
		return
	}

	req.Header.Set("Range", formattedRangeHeader)

	fmt.Fprintf(d.outStream, "start GET request with header: \"Range: %s\"\n", formattedRangeHeader)

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		errCh <- err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		errCh <- fmt.Errorf("unexpected response: status code: %d", resp.StatusCode)
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
	cleanFn := func() { os.RemoveAll(dir) }
	interruptor.RegisterCleanFunction(cleanFn)
	return dir, cleanFn, nil
}

func genUUID() string {
	u, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return u.String()
}
