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
	"path"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	errExist = errors.New("file already exists")
)

func main() {
	opts, url := parse(os.Args[1:]...)
	err := newDownloader(os.Stdout, url, opts).download()
	if err != nil {
		log.Fatal(err)
	}
}

func parse(args ...string) (*options, string) {
	flg := flag.NewFlagSet("parallel-download", flag.ExitOnError)
	parallelism := flg.Int("p", 8, "parallelism")
	timeout := flg.Int("t", 60, "timeout")
	output := flg.String("o", "", "output")
	flg.Parse(args)
	url := flg.Arg(0)
	return &options{parallelism: *parallelism, timeout: time.Duration(*timeout) * time.Second, output: *output}, url
}

type options struct {
	parallelism int
	timeout     time.Duration
	output      string
}

type downloader struct {
	outStream   io.Writer
	url         string
	parallelism int
	timeout     time.Duration
	output      string
}

func newDownloader(w io.Writer, url string, opts *options) *downloader {
	return &downloader{outStream: w, url: url, parallelism: opts.parallelism, timeout: opts.timeout, output: opts.output}
}

func (d *downloader) download() error {
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

	resps, err := d.doRequest(rangeStrings)
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

func (d *downloader) doRequest(rangeStrings []string) (map[int]*http.Response, error) {
	resps := map[int]*http.Response{}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	var m sync.Mutex
	eg, ctx := errgroup.WithContext(ctx)
	for i, rangeString := range rangeStrings {
		i := i
		rangeString := rangeString
		eg.Go(func() error {
			resp, err := d.doRangeRequest(ctx, rangeString)
			if err != nil {
				return err
			}
			fmt.Fprintf(d.outStream, "i: %d, ContentLength: %d, Range: %s\n", i, resp.ContentLength, rangeString)
			m.Lock()
			resps[i] = resp
			m.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return resps, nil
}

func (d *downloader) doRangeRequest(ctx context.Context, rangeString string) (*http.Response, error) {
	client := &http.Client{Timeout: 0}

	req, err := http.NewRequest("GET", d.url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	req.Header.Set("Range", rangeString)

	fmt.Fprintf(d.outStream, "Start requesting %q ...\n", rangeString)
	resp, err := client.Do(req)
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
