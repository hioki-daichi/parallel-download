package downloading

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hioki-daichi/parallel-download/opt"
)

var registeredTestdatum = map[string]string{
	"foo.png":   readTestdata("foo.png"),
	"a.txt":     readTestdata("a.txt"),
	"empty.txt": readTestdata("empty.txt"),
}

var currentTestdataName string

func TestDownloading_Download_Success(t *testing.T) {
	cases := map[string]struct {
		parallelism         int
		currentTestdataName string
	}{
		"normal":                      {parallelism: 3, currentTestdataName: "foo.png"},
		"parallelism < 1":             {parallelism: 0, currentTestdataName: "a.txt"},
		"contentLength < parallelism": {parallelism: 4, currentTestdataName: "a.txt"},
	}

	for n, c := range cases {
		c := c
		t.Run(n, func(t *testing.T) {
			parallelism := c.parallelism
			currentTestdataName = c.currentTestdataName

			output, clean := createTempOutput(t)
			defer clean()

			ts, clean := newTestServer(t, normalHandler)
			defer clean()

			err := newDownloader(t, output, ts, parallelism).Download(context.Background())
			if err != nil {
				t.Fatalf("err %s", err)
			}
		})
	}

}

func TestDownloading_Download_NoContent(t *testing.T) {
	expected := errNoContent

	currentTestdataName = "empty.txt"

	output, clean := createTempOutput(t)
	defer clean()

	ts, clean := newTestServer(t, normalHandler)
	defer clean()

	actual := newDownloader(t, output, ts, 1).Download(context.Background())
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestDownloading_Download_AcceptRangesHeaderNotFound(t *testing.T) {
	expected := errResponseDoesNotIncludeAcceptRangesHeader

	output, clean := createTempOutput(t)
	defer clean()

	ts, clean := newTestServer(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "") })
	defer clean()

	actual := newDownloader(t, output, ts, 8).Download(context.Background())
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestDownloading_Download_AcceptRangesHeaderSupportsBytesOnly(t *testing.T) {
	expected := errValueOfAcceptRangesHeaderIsNotBytes

	output, clean := createTempOutput(t)
	defer clean()

	ts, clean := newTestServer(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "none")
		fmt.Fprint(w, "")
	})
	defer clean()

	actual := newDownloader(t, output, ts, 8).Download(context.Background())
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestDownloading_Download_BadRequest(t *testing.T) {
	expected := "unexpected response: status code: 400"

	output, clean := createTempOutput(t)
	defer clean()

	ts, clean := newTestServer(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "bad request")
	})
	defer clean()

	err := newDownloader(t, output, ts, 8).Download(context.Background())
	if err == nil {
		t.Fatalf("unexpectedly err is nil")
	}
	actual := err.Error()
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func newTestServer(t *testing.T, handler func(t *testing.T, w http.ResponseWriter, r *http.Request)) (*httptest.Server, func()) {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			handler(t, w, r)
		},
	))

	return ts, func() { ts.Close() }
}

func normalHandler(t *testing.T, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Accept-Ranges", "bytes")

	rangeHdr := r.Header.Get("Range")

	body := func() string {
		if rangeHdr == "" {
			return registeredTestdatum[currentTestdataName]
		}

		eqlSplitVals := strings.Split(rangeHdr, "=")
		if eqlSplitVals[0] != "bytes" {
			t.Fatalf("err %s", eqlSplitVals[0])
		}

		c := strings.Split(eqlSplitVals[1], "-")

		min, err := strconv.Atoi(c[0])
		if err != nil {
			t.Fatalf("err %s", err)
		}

		max, err := strconv.Atoi(c[1])
		if err != nil {
			t.Fatalf("err %s", err)
		}

		return registeredTestdatum[currentTestdataName][min : max+1]
	}()

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))

	w.WriteHeader(http.StatusPartialContent)

	fmt.Fprint(w, body)
}

func newDownloader(t *testing.T, output string, ts *httptest.Server, parallelism int) *Downloader {
	opts := &opt.Options{
		Parallelism: parallelism,
		Output:      output,
		URL:         mustParseRequestURI(t, ts.URL),
		Timeout:     60 * time.Second,
	}

	return NewDownloader(ioutil.Discard, opts)
}

func mustParseRequestURI(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.ParseRequestURI(s)
	if err != nil {
		t.Fatalf("err %s", err)
	}
	return u
}

func readTestdata(filename string) string {
	b, err := ioutil.ReadFile(path.Join("testdata", filename))
	if err != nil {
		panic(err)
	}
	return string(b)
}

func createTempOutput(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := ioutil.TempDir("", "parallel-download")
	if err != nil {
		panic(err)
	}

	return filepath.Join(dir, "output.txt"), func() { os.RemoveAll(dir) }
}
