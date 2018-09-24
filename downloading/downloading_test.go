package downloading

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hioki-daichi/parallel-download/opt"
)

var contents = func() string {
	b, err := ioutil.ReadFile("../testdata/foo.png")
	if err != nil {
		panic(err)
	}
	return string(b)
}()

func TestDownloading_Download(t *testing.T) {
	fp, err := ioutil.TempFile("", "parallel-download")
	if err != nil {
		t.Fatalf("err %s", err)
	}
	defer os.Remove(fp.Name())

	ts, clean := newTestServer(t, normalHandler)
	defer clean()

	opts := &opt.Options{
		Parallelism: 8,
		DstFile:     fp,
		URL:         mustParseRequestURI(t, ts.URL),
	}

	d := NewDownloader(ioutil.Discard, opts)

	err = d.Download(context.Background())
	if err != nil {
		t.Fatalf("err %s", err)
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
			return contents
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

		return contents[min : max+1]
	}()

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))

	fmt.Fprint(w, body)
}

func mustParseRequestURI(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.ParseRequestURI(s)
	if err != nil {
		t.Fatalf("err %s", err)
	}
	return u
}
