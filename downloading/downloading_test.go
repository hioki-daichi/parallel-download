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
	"strconv"
	"strings"
	"testing"

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

			fp, clean := createDstFile(t)
			defer clean()

			ts, clean := newTestServer(t, normalHandler)
			defer clean()

			err := newDownloader(t, fp, ts, parallelism).Download(context.Background())
			if err != nil {
				t.Fatalf("err %s", err)
			}
		})
	}

}

func TestDownloading_Download_NoContent(t *testing.T) {
	expected := errNoContent

	currentTestdataName = "empty.txt"

	fp, clean := createDstFile(t)
	defer clean()

	ts, clean := newTestServer(t, normalHandler)
	defer clean()

	actual := newDownloader(t, fp, ts, 1).Download(context.Background())
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestDownloading_Download_AcceptRangesHeaderNotFound(t *testing.T) {
	expected := errAcceptRangesHeaderNotFound

	fp, clean := createDstFile(t)
	defer clean()

	ts, clean := newTestServer(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "") })
	defer clean()

	actual := newDownloader(t, fp, ts, 8).Download(context.Background())
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestDownloading_Download_AcceptRangesHeaderSupportsBytesOnly(t *testing.T) {
	expected := errAcceptRangesHeaderSupportsBytesOnly

	fp, clean := createDstFile(t)
	defer clean()

	ts, clean := newTestServer(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "none")
		fmt.Fprint(w, "")
	})
	defer clean()

	actual := newDownloader(t, fp, ts, 8).Download(context.Background())
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestDownloading_Download_BadRequest(t *testing.T) {
	expected := "unexpected response: status code: 400"

	fp, clean := createDstFile(t)
	defer clean()

	ts, clean := newTestServer(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "bad request")
	})
	defer clean()

	err := newDownloader(t, fp, ts, 8).Download(context.Background())
	if err == nil {
		t.Fatalf("unexpectedly err is nil")
	}
	actual := err.Error()
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestDownloading_concat_NonExistentFilename(t *testing.T) {
	expected := "open non-existent: no such file or directory"

	ts, clean := newTestServer(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {})
	defer clean()

	d := NewDownloader(ioutil.Discard, &opt.Options{Parallelism: 1, DstFile: nil, URL: mustParseRequestURI(t, ts.URL)})

	err := d.concat(map[int]string{0: "non-existent"})

	actual := err.Error()
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestDownloading_concat_DstFileError(t *testing.T) {
	expected := "invalid argument"

	ts, clean := newTestServer(t, func(t *testing.T, w http.ResponseWriter, r *http.Request) {})
	defer clean()

	d := NewDownloader(ioutil.Discard, &opt.Options{Parallelism: 1, DstFile: nil, URL: mustParseRequestURI(t, ts.URL)})

	err := d.concat(map[int]string{0: "testdata/a.txt", 1: "testdata/b.txt"})

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

	fmt.Fprint(w, body)
}

func createDstFile(t *testing.T) (*os.File, func()) {
	fp, err := ioutil.TempFile("", "parallel-download")
	if err != nil {
		t.Fatalf("err %s", err)
	}
	return fp, func() { os.Remove(fp.Name()) }
}

func newDownloader(t *testing.T, fp *os.File, ts *httptest.Server, parallelism int) *Downloader {
	opts := &opt.Options{
		Parallelism: parallelism,
		DstFile:     fp,
		URL:         mustParseRequestURI(t, ts.URL),
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
