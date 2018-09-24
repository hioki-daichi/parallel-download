package opt

import (
	"net/url"
	"os"
	"reflect"
	"testing"
)

func TestMain_parse(t *testing.T) {
	cases := map[string]struct {
		args                []string
		expectedParallelism int
		expectedFilename    string
		expectedURLString   string
	}{
		"no options": {args: []string{"http://example.com/foo.png"}, expectedParallelism: 8, expectedFilename: "foo.png", expectedURLString: "http://example.com/foo.png"},
		"-p=2":       {args: []string{"-p=2", "http://example.com/foo.png"}, expectedParallelism: 2, expectedFilename: "foo.png", expectedURLString: "http://example.com/foo.png"},
		"-o=bar.png": {args: []string{"-o=bar.png", "http://example.com/foo.png"}, expectedParallelism: 8, expectedFilename: "bar.png", expectedURLString: "http://example.com/foo.png"},
		"index.html": {args: []string{"http://example.com/"}, expectedParallelism: 8, expectedFilename: "index.html", expectedURLString: "http://example.com/"},
	}

	for n, c := range cases {
		c := c
		t.Run(n, func(t *testing.T) {
			args := c.args
			expectedParallelism := c.expectedParallelism
			expectedFilename := c.expectedFilename
			expectedURL, err := url.ParseRequestURI(c.expectedURLString)
			if err != nil {
				t.Fatalf("err %s", err)
			}

			opts, err := Parse(args...)
			if err != nil {
				t.Fatalf("err %s", err)
			}

			actualParallelism := opts.Parallelism
			actualFilename := opts.DstFile.Name()
			actualURL := opts.URL

			if actualParallelism != expectedParallelism {
				t.Errorf(`unexpected parallelism: expected: %d actual: %d`, expectedParallelism, actualParallelism)
			}

			if actualFilename != expectedFilename {
				t.Errorf(`unexpected output: expected: "%s" actual: "%s"`, expectedFilename, actualFilename)
			}
			err = os.Remove(expectedFilename)
			if err != nil {
				t.Fatalf("err %s", err)
			}

			if !reflect.DeepEqual(actualURL, expectedURL) {
				t.Errorf(`unexpected URL: expected: "%s" actual: "%s"`, expectedURL, actualURL)
			}
		})
	}
}

func TestMain_parse_InvalidURL(t *testing.T) {
	t.Parallel()

	expected := "parse %: invalid URI for request"

	_, err := Parse([]string{"%"}...)
	if err == nil {
		t.Fatal("Unexpectedly err was nil")
	}

	actual := err.Error()
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}

func TestMain_parse_FileAlreadyExists(t *testing.T) {
	t.Parallel()

	expected := "open foo.png: file exists"

	_, err := os.Create("foo.png")
	if err != nil {
		t.Fatalf("err %s", err)
	}
	defer os.Remove("foo.png")

	_, err = Parse([]string{"http://example.com/foo.png"}...)
	if err == nil {
		t.Fatal("Unexpectedly err was nil")
	}

	actual := err.Error()
	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}
