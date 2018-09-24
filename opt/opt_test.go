package opt

import (
	"net/url"
	"reflect"
	"testing"
)

func TestMain_parse(t *testing.T) {
	cases := map[string]struct {
		args                []string
		expectedParallelism int
		expectedOutput      string
		expectedURLString   string
	}{
		"no options": {args: []string{"http://example.com/foo.png"}, expectedParallelism: 8, expectedOutput: "", expectedURLString: "http://example.com/foo.png"},
		"-p=2":       {args: []string{"-p=2", "http://example.com/foo.png"}, expectedParallelism: 2, expectedOutput: "", expectedURLString: "http://example.com/foo.png"},
		"-o=bar.png": {args: []string{"-o=bar.png", "http://example.com/foo.png"}, expectedParallelism: 8, expectedOutput: "bar.png", expectedURLString: "http://example.com/foo.png"},
	}

	for n, c := range cases {
		c := c
		t.Run(n, func(t *testing.T) {
			t.Parallel()

			args := c.args
			expectedParallelism := c.expectedParallelism
			expectedOutput := c.expectedOutput
			expectedURL, err := url.ParseRequestURI(c.expectedURLString)
			if err != nil {
				t.Fatalf("err %s", err)
			}

			opts, err := Parse(args...)

			actualParallelism := opts.Parallelism
			actualOutput := opts.Output
			actualURL := opts.URL

			if actualParallelism != expectedParallelism {
				t.Errorf(`unexpected parallelism: expected: %d actual: %d`, expectedParallelism, actualParallelism)
			}

			if actualOutput != expectedOutput {
				t.Errorf(`unexpected output: expected: "%s" actual: "%s"`, expectedOutput, actualOutput)
			}

			if !reflect.DeepEqual(actualURL, expectedURL) {
				t.Errorf(`unexpected URL: expected: "%s" actual: "%s"`, expectedURL, actualURL)
			}
		})
	}
}

func TestMain_parse_InvalidURL(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"%"}...)
	if err == nil {
		t.Fatal("Unexpectedly err was nil")
	}

	actual := err.Error()
	expected := "parse %: invalid URI for request"

	if actual != expected {
		t.Errorf(`unexpected error: expected: "%s" actual: "%s"`, expected, actual)
	}
}
