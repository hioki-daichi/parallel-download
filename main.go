package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/hioki-daichi/parallel-download/interruptor"
	"github.com/hioki-daichi/parallel-download/opt"
	"golang.org/x/sync/errgroup"
)

var (
	errExist = errors.New("file already exists")
)

func main() {
	err := execute(os.Args[1:], os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}

func execute(args []string, w io.Writer) error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interruptor.RegisterCleanFunction(cancel)
	interruptor.Setup()

	opts, err := opt.Parse(args...)
	if err != nil {
		return err
	}

	d := newDownloader(w, opts)
	err = d.download(ctx)
	if err != nil {
		return err
	}
	return nil
}

type downloader struct {
	outStream   io.Writer
	url         *url.URL
	parallelism int
	output      string
}

func newDownloader(w io.Writer, opts *opt.Options) *downloader {
	return &downloader{
		outStream:   w,
		url:         opts.URL,
		parallelism: opts.Parallelism,
		output:      opts.Output,
	}
}

func (d *downloader) download(ctx context.Context) error {
	filename, err := d.genFilename()
	if err != nil {
		return err
	}

	resp, err := http.Head(d.url.String())
	if err != nil {
		return err
	}

	rangeStrings, err := generateFormattedRangeHeaders(int(resp.ContentLength), d.parallelism)
	if err != nil {
		return err
	}

	tempDir, err := ioutil.TempDir("", "parallel-download")
	if err != nil {
		return err
	}
	cleanTempDir := func() { os.RemoveAll(tempDir) }
	defer cleanTempDir()
	interruptor.RegisterCleanFunction(cleanTempDir)

	chunks, err := d.doRequest(ctx, rangeStrings, tempDir)
	if err != nil {
		return err
	}

	dst, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	err = concatChunks(dst, chunks)
	if err != nil {
		os.Remove(dst.Name())
		return err
	}

	fmt.Fprintf(d.outStream, "%q saved\n", dst.Name())

	return nil
}

// If output is specified, it will be used, otherwise use the one generated from URL.
func (d *downloader) genFilename() (string, error) {
	if d.output != "" {
		return d.output, nil
	}

	_, filename := path.Split(d.url.Path)

	// Inspired by the --default-page option of wget
	if filename == "" {
		filename = "index.html"
	}

	_, err := os.Lstat(filename)
	if err == nil {
		return "", errExist
	}

	return filename, nil
}

func (d *downloader) doRequest(ctx context.Context, rangeStrings []string, dir string) (map[int]string, error) {
	ch := make(chan map[int]string)
	errCh := make(chan error)

	for i, rangeString := range rangeStrings {
		i := i
		rangeString := rangeString
		go func() {
			resp, err := d.doRangeRequest(ctx, rangeString)
			if err != nil {
				errCh <- err
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
			resp.Body.Close()
			ch <- map[int]string{i: tmp.Name()}
			return
		}()
	}

	chunks := map[int]string{}

	eg, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	for i := 0; i < len(rangeStrings); i++ {
		eg.Go(func() error {
			select {
			case <-ctx.Done():
				return nil
			case m := <-ch:
				fmt.Fprintln(d.outStream, m)
				for k, v := range m {
					mu.Lock()
					chunks[k] = v
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

	return chunks, nil
}

func (d *downloader) doRangeRequest(ctx context.Context, rangeString string) (*http.Response, error) {
	req, err := http.NewRequest("GET", d.url.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Range", rangeString)

	fmt.Fprintf(d.outStream, "Start requesting %q ...\n", rangeString)
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unexpected response: status code: " + strconv.Itoa(resp.StatusCode))
	}

	return resp, nil
}

func generateFormattedRangeHeaders(contentLength int, parallelism int) ([]string, error) {
	if parallelism < 1 {
		parallelism = 1
	}

	if contentLength < parallelism {
		parallelism = contentLength
	}

	chunkLen := contentLength / parallelism
	rem := contentLength % parallelism

	ss := make([]string, 0)

	cntr := 0
	for n := parallelism; n > 0; n-- {
		min := cntr
		max := cntr + chunkLen - 1

		if n == 1 && rem != 0 {
			max += rem
		}

		ss = append(ss, fmt.Sprintf("bytes=%d-%d", min, max))

		cntr += chunkLen
	}

	return ss, nil
}

func concatChunks(dst *os.File, chunks map[int]string) error {
	for i := 0; i < len(chunks); i++ {
		chunk := chunks[i]
		src, err := os.Open(chunk)
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, src)
		if err != nil {
			return err
		}
	}

	return nil
}
