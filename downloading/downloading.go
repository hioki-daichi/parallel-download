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
	"sync"

	"github.com/hioki-daichi/parallel-download/interruptor"
	"github.com/hioki-daichi/parallel-download/opt"
	"golang.org/x/sync/errgroup"
)

var (
	errAcceptRangesHeaderNotFound          = errors.New("Accept-Ranges header not found")
	errAcceptRangesHeaderSupportsBytesOnly = errors.New("Accept-Ranges header supports bytes only")
)

// Downloader has the information for the download.
type Downloader struct {
	outStream   io.Writer
	url         *url.URL
	parallelism int
	dstFile     *os.File
}

// NewDownloader generates Downloader based on Options.
func NewDownloader(w io.Writer, opts *opt.Options) *Downloader {
	return &Downloader{
		outStream:   w,
		url:         opts.URL,
		parallelism: opts.Parallelism,
		dstFile:     opts.DstFile,
	}
}

// Download performs parallel download.
func (d *Downloader) Download(ctx context.Context) error {
	contentLength, err := d.validateHeaderAndGetContentLength()
	if err != nil {
		return err
	}

	formattedRangeHeaders, err := d.genFormattedRangeHeaders(contentLength)
	if err != nil {
		return err
	}

	dir, cleanFn, err := createWorkDir()
	if err != nil {
		return err
	}
	defer cleanFn()

	filenameCh := make(chan map[int]string)
	errCh := make(chan error)
	for i, frh := range formattedRangeHeaders {
		go d.downloadChunkFile(ctx, i, frh, filenameCh, errCh, dir)
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

	err = d.concat(filenames)
	if err != nil {
		return err
	}

	fmt.Fprintf(d.outStream, "%q saved\n", d.dstFile.Name())

	return nil
}

func (d *Downloader) validateHeaderAndGetContentLength() (int, error) {
	resp, err := http.Head(d.url.String())
	if err != nil {
		return 0, err
	}

	acceptRangesHeader := resp.Header.Get("Accept-Ranges")
	if acceptRangesHeader == "" {
		return 0, errAcceptRangesHeaderNotFound
	}

	if acceptRangesHeader != "bytes" {
		return 0, errAcceptRangesHeaderSupportsBytesOnly
	}

	return int(resp.ContentLength), nil
}

func (d *Downloader) genFormattedRangeHeaders(contentLength int) ([]string, error) {
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

	return ss, nil
}

func (d *Downloader) downloadChunkFile(ctx context.Context, i int, formattedRangeHeader string, ch chan<- map[int]string, errCh chan<- error, dir string) {
	req, err := http.NewRequest("GET", d.url.String(), nil)
	if err != nil {
		errCh <- err
		return
	}

	req.Header.Set("Range", formattedRangeHeader)

	fmt.Fprintf(d.outStream, "Start requesting %q ...\n", formattedRangeHeader)

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		errCh <- err
		return
	}

	if resp.StatusCode != http.StatusOK {
		errCh <- fmt.Errorf("unexpected response: status code: %d", resp.StatusCode)
		return
	}

	tmp, err := os.Create(path.Join(dir, fmt.Sprintf("%02d", i)))
	if err != nil {
		errCh <- err
		return
	}

	_, err = io.Copy(tmp, resp.Body)
	if err != nil {
		errCh <- err
		return
	}

	fmt.Fprintf(d.outStream, "Download chunked file %q\n", tmp.Name())

	resp.Body.Close()

	ch <- map[int]string{i: tmp.Name()}

	return
}

func (d *Downloader) concat(filenames map[int]string) error {
	for i := 0; i < len(filenames); i++ {
		filename := filenames[i]
		src, err := os.Open(filename)
		if err != nil {
			return err
		}
		_, err = io.Copy(d.dstFile, src)
		if err != nil {
			return err
		}
		err = src.Close()
		if err != nil {
			return err
		}
	}

	d.dstFile.Close()

	return nil
}

func createWorkDir() (string, func(), error) {
	dir, err := ioutil.TempDir("", "parallel-download")
	if err != nil {
		return "", nil, err
	}
	cleanFn := func() { os.RemoveAll(dir) }
	interruptor.RegisterCleanFunction(cleanFn)
	return dir, cleanFn, nil
}
