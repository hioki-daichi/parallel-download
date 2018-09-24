package opt

import (
	"reflect"
	"testing"
)

func TestMain_parse(t *testing.T) {
	cases := map[string]struct {
		args         []string
		expectedOpts *Options
		expectedURL  string
	}{
		"no options":         {args: []string{"http://example.com/foo.png"}, expectedOpts: &Options{Parallelism: 8, Output: ""}, expectedURL: "http://example.com/foo.png"},
		"Parallelism option": {args: []string{"-p=2", "http://example.com/foo.png"}, expectedOpts: &Options{Parallelism: 2, Output: ""}, expectedURL: "http://example.com/foo.png"},
		"Output option":      {args: []string{"-o=bar.png", "http://example.com/foo.png"}, expectedOpts: &Options{Parallelism: 8, Output: "bar.png"}, expectedURL: "http://example.com/foo.png"},
	}

	for n, c := range cases {
		c := c
		t.Run(n, func(t *testing.T) {
			t.Parallel()

			args := c.args
			expectedOpts := c.expectedOpts
			expectedURL := c.expectedURL

			opts, url := Parse(args...)
			if !reflect.DeepEqual(opts, expectedOpts) {
				t.Errorf(`unexpected *options: expected: %v actual: %v`, expectedOpts, opts)
			}
			if url != expectedURL {
				t.Errorf(`unexpected url: expected: "%s" actual: "%s"`, expectedURL, url)
			}
		})
	}
}
