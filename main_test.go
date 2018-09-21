package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestMain_parse(t *testing.T) {
	cases := map[string]struct {
		args         []string
		expectedOpts *options
		expectedURL  string
	}{
		"no options":         {args: []string{"http://example.com/foo.png"}, expectedOpts: &options{parallelism: 8, output: ""}, expectedURL: "http://example.com/foo.png"},
		"parallelism option": {args: []string{"-p=2", "http://example.com/foo.png"}, expectedOpts: &options{parallelism: 2, output: ""}, expectedURL: "http://example.com/foo.png"},
		"output option":      {args: []string{"-o=bar.png", "http://example.com/foo.png"}, expectedOpts: &options{parallelism: 8, output: "bar.png"}, expectedURL: "http://example.com/foo.png"},
	}

	for n, c := range cases {
		c := c
		t.Run(n, func(t *testing.T) {
			args := c.args
			expectedOpts := c.expectedOpts
			expectedURL := c.expectedURL

			opts, url := parse(args...)
			if !reflect.DeepEqual(opts, expectedOpts) {
				t.Errorf(`unexpected *options: expected: %v actual: %v`, expectedOpts, opts)
			}
			if url != expectedURL {
				t.Errorf(`unexpected url: expected: "%s" actual: "%s"`, expectedURL, url)
			}
		})
	}
}

func TestMain_download(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	dir, err := ioutil.TempDir("", "parallel-download")
	if err != nil {
		t.Fatalf("err %s", err)
	}
	defer os.Remove(dir)

	err = newDownloader(ioutil.Discard, ts.URL, &options{parallelism: 8, output: filepath.Join(dir, "foo.png")}).download()
	if err != nil {
		t.Fatalf("err %s", err)
	}
}

func TestMain_genFilename(t *testing.T) {
	cases := map[string]struct {
		url      string
		output   string
		expected string
	}{
		"index.html": {url: "http://example.com", output: "", expected: "index.html"},
		"foo.png":    {url: "http://example.com/foo.png", output: "", expected: "foo.png"},
	}

	for n, c := range cases {
		c := c
		t.Run(n, func(t *testing.T) {
			t.Parallel()

			expected := c.expected
			output := c.output
			url := c.url

			actual, err := newDownloader(ioutil.Discard, url, &options{parallelism: 8, output: output}).genFilename()
			if err != nil {
				t.Fatalf("err %s", err)
			}
			if actual != expected {
				t.Errorf(`unexpected filename: expected: "%s" actual: "%s"`, expected, actual)
			}
		})
	}
}

func TestMain_genFilename_ParseError(t *testing.T) {
	expected := `parse %: invalid URL escape "%"`
	d := newDownloader(ioutil.Discard, "%", &options{parallelism: 8, output: ""})
	_, err := d.genFilename()
	actual := err.Error()
	if actual != expected {
		t.Errorf(`unexpected : expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestMain_genFilename_IsNotExist(t *testing.T) {
	expected := errExist

	d := newDownloader(ioutil.Discard, "http://example.com/main_test.go", &options{parallelism: 8, output: ""})
	_, actual := d.genFilename()
	if actual != expected {
		t.Errorf(`unexpected : expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestMain_toRangeStrings(t *testing.T) {
	cases := map[string]struct {
		contentLength int
		parallelism   int
		expected      []string
	}{
		"contentLength: 5, parallelism: 0": {contentLength: 5, parallelism: 0, expected: []string{"bytes=0-4"}},
		"contentLength: 5, parallelism: 1": {contentLength: 5, parallelism: 1, expected: []string{"bytes=0-4"}},
		"contentLength: 5, parallelism: 2": {contentLength: 5, parallelism: 2, expected: []string{"bytes=0-1", "bytes=2-4"}},
		"contentLength: 5, parallelism: 3": {contentLength: 5, parallelism: 3, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-4"}},
		"contentLength: 5, parallelism: 4": {contentLength: 5, parallelism: 4, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-2", "bytes=3-4"}},
		"contentLength: 5, parallelism: 5": {contentLength: 5, parallelism: 5, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-2", "bytes=3-3", "bytes=4-4"}},
		"contentLength: 5, parallelism: 6": {contentLength: 5, parallelism: 6, expected: []string{"bytes=0-0", "bytes=1-1", "bytes=2-2", "bytes=3-3", "bytes=4-4"}},
	}

	for n, c := range cases {
		c := c
		t.Run(n, func(t *testing.T) {
			t.Parallel()

			contentLength := c.contentLength
			parallelism := c.parallelism
			expected := c.expected

			actual, err := toRangeStrings(contentLength, parallelism)
			if err != nil {
				t.Fatalf("err %s", err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf(`expected="%s" actual="%s"`, expected, actual)
			}
		})
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Accept-Ranges", "bytes")
			// body, err := genBody(t, r)

			body, err := func(t *testing.T, req *http.Request) (string, error) {
				b, err := ioutil.ReadFile("./testdata/foo.png")
				if err != nil {
					t.Fatalf("err %s", err)
				}
				contents := string(b)

				// e.g. "bytes=0-99"
				rangeHdr := req.Header.Get("Range")
				if rangeHdr == "" {
					return contents, nil
				}

				// e.g. []string{"bytes", "0-99"}
				eqlSplitVals := strings.Split(rangeHdr, "=")
				if eqlSplitVals[0] != "bytes" {
					return "", errors.New(`only "bytes" is accepted`)
				}

				// e.g. []string{"0", "99"}
				c := strings.Split(eqlSplitVals[1], "-")

				// e.g. 0
				start, err := strconv.Atoi(c[0])
				if err != nil {
					return "", err
				}

				// e.g. 99
				end, err := strconv.Atoi(c[1])
				if err != nil {
					return "", err
				}

				// e.g. "Range: bytes=1-0"
				if start > end {
					return "", errors.New("invalid range")
				}

				l := len(contents)
				if end > l {
					end = l
				}

				return contents[start : end+1], nil
			}(t, r)

			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, err.Error())
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			fmt.Fprint(w, body)
			return
		},
	))
}
