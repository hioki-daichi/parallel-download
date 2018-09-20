package main

import (
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

	responses := map[int]*http.Response{}

	var m sync.Mutex
	eg := errgroup.Group{}
	for i, rangeString := range rangeStrings {
		i := i
		rangeString := rangeString
		eg.Go(func() error {
			client := &http.Client{Timeout: 0}

			req, err := http.NewRequest("GET", d.url, nil)
			if err != nil {
				return err
			}

			req.Header.Set("Range", rangeString)

			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			fmt.Fprintf(d.outStream, "i: %d, ContentLength: %d, Range: %s\n", i, resp.ContentLength, rangeString)
			if resp.StatusCode != http.StatusOK {
				return errors.New("unexpected response: status code: " + strconv.Itoa(resp.StatusCode))
			}
			m.Lock()
			responses[i] = resp
			m.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	fp, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	for i := 0; i < len(responses); i++ {
		resp := responses[i]
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

	_, err = os.Stat(filename)
	if !os.IsNotExist(err) {
		return "", errExist
	}

	return filename, nil
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
