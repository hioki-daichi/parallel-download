package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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

	go printNumGoroutineLoop()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	setupCloseHandler(cancel)
	opts, url := parse(os.Args[1:]...)
	err := newDownloader(os.Stdout, url, opts).download(ctx)
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)
}

func printNumGoroutineLoop() {
	for {
		fmt.Printf("num goroutine: %d\n", runtime.NumGoroutine())
		time.Sleep(100 * time.Millisecond)
	}
}

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

func parse(args ...string) (*options, string) {
	flg := flag.NewFlagSet("parallel-download", flag.ExitOnError)
	parallelism := flg.Int("p", 8, "parallelism")
	output := flg.String("o", "", "output")
	flg.Parse(args)
	url := flg.Arg(0)
	return &options{parallelism: *parallelism, output: *output}, url
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
	return &downloader{outStream: w, url: url, parallelism: opts.parallelism, output: opts.output}
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

	resps, err := d.doRequest(ctx, rangeStrings)
	if err != nil {
		return err
	}

	fp, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	for i := 0; i < len(resps); i++ {
		resp := resps[i]
		_, err := io.Copy(fp, resp.Body)
		if err != nil {
			os.Remove(filename)
			return err
		}
	}

	fmt.Fprintf(d.outStream, "%q saved\n", filename)

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

func (d *downloader) doRequest(ctx context.Context, rangeStrings []string) (map[int]*http.Response, error) {
	log.Print("... doRequest start")
	defer log.Print("... doRequest end")

	ch := make(chan map[int]*http.Response)
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
			fmt.Fprintf(d.outStream, "i: %d, ContentLength: %d, Range: %s\n", i, resp.ContentLength, rangeString)
			ch <- map[int]*http.Response{i: resp}
			return
		}()
	}

	resps := map[int]*http.Response{}

	eg, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	for i := 0; i < len(rangeStrings); i++ {
		eg.Go(func() error {
			select {
			case <-ctx.Done():
				fmt.Println("ctx.Done() in eg.Go")
				return nil
			case m := <-ch:
				for k, v := range m {
					mu.Lock()
					resps[k] = v
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

	return resps, nil
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
