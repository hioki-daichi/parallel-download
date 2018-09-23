package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	errExist = errors.New("file already exists")
)

func main() {
	log.Print("... main start")
	defer log.Print("... main end")

	// for debug
	go printNumGoroutineLoop()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	setupCloseHandler(cancel)

	opts, url := parse(os.Args[1:]...)

	d := newDownloader(os.Stdout, url, opts)
	err := d.download(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// for debug
	time.Sleep(100 * time.Millisecond)
}

// for debug
func printNumGoroutineLoop() {
	for {
		fmt.Printf("num goroutine: %d\n", runtime.NumGoroutine())
		time.Sleep(100 * time.Millisecond)
	}
}

// setupCloseHandler handles Ctrl+C
func setupCloseHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		cancel()
		os.Exit(0)
	}()
}

// parse parses command line options
func parse(args ...string) (*options, string) {
	flg := flag.NewFlagSet("parallel-download", flag.ExitOnError)

	parallelism := flg.Int("p", 8, "parallelism")
	output := flg.String("o", "", "output")

	flg.Parse(args)

	url := flg.Arg(0)

	return &options{
		parallelism: *parallelism,
		output:      *output,
	}, url
}

type options struct {
	parallelism int
	output      string
}

type downloader struct {
	outStream   io.Writer
	url         string
	parallelism int
	output      string
}

func newDownloader(w io.Writer, url string, opts *options) *downloader {
	return &downloader{
		outStream:   w,
		url:         url,
		parallelism: opts.parallelism,
		output:      opts.output,
	}
}

func (d *downloader) download(ctx context.Context) error {
	log.Print("... download start")
	defer log.Print("... download end")

	filename, err := d.genFilename()
	if err != nil {
		return err
	}

	resp, err := http.Head(d.url)
	if err != nil {
		return err
	}

	rangeStrings, err := toRangeStrings(int(resp.ContentLength), d.parallelism)
	if err != nil {
		return err
	}

	tempDir, err := ioutil.TempDir("", "parallel-download")
	if err != nil {
		return err
	}
	fmt.Println(tempDir)
	defer os.RemoveAll(tempDir)

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

func (d *downloader) genFilename() (string, error) {
	if d.output != "" {
		return d.output, nil
	}

	u, err := url.Parse(d.url)
	if err != nil {
		return "", err
	}
	_, filename := path.Split(u.Path)

	if filename == "" {
		filename = "index.html"
	}

	_, err = os.Lstat(filename)
	if err == nil {
		return "", errExist
	}

	return filename, nil
}

func (d *downloader) doRequest(ctx context.Context, rangeStrings []string, dir string) (map[int]string, error) {
	log.Print("... doRequest start")
	defer log.Print("... doRequest end")

	ch := make(chan map[int]string)
	errCh := make(chan error)

	for i, rangeString := range rangeStrings {
		i := i
		rangeString := rangeString
		go func() {
			resp, err := d.doRangeRequest(rangeString)
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
				fmt.Fprintln(d.outStream, "ctx.Done() in eg.Go")
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

func (d *downloader) doRangeRequest(rangeString string) (*http.Response, error) {
	log.Print("... doRangeRequest start")
	defer log.Print("... doRangeRequest end")

	req, err := http.NewRequest("GET", d.url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Range", rangeString)

	fmt.Fprintf(d.outStream, "Start requesting %q ...\n", rangeString)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unexpected response: status code: " + strconv.Itoa(resp.StatusCode))
	}

	return resp, nil
}

func toRangeStrings(contentLength int, parallelism int) ([]string, error) {
	rangeStructs := make([]rangeStruct, 0)

	if parallelism == 0 {
		parallelism = 1
	}

	if contentLength < parallelism {
		parallelism = contentLength
	}

	length := contentLength / parallelism

	i := 0
	for n := parallelism; n > 0; n-- {
		first := i
		i += length
		last := i - 1
		rangeStructs = append(rangeStructs, rangeStruct{first: first, last: last})
	}

	if rem := contentLength % parallelism; rem != 0 {
		rangeStructs[len(rangeStructs)-1].last += rem
	}

	rangeStrings := make([]string, 0)

	for _, rangeStruct := range rangeStructs {
		rangeStrings = append(rangeStrings, fmt.Sprintf("bytes=%d-%d", rangeStruct.first, rangeStruct.last))
	}

	return rangeStrings, nil
}

type rangeStruct struct {
	first int
	last  int
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
