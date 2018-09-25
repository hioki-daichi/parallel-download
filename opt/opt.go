/*
Package opt deals with CLI options.
*/
package opt

import (
	"errors"
	"flag"
	"net/url"
	"path"
)

var errExist = errors.New("file already exists")

// Options has the options required for parallel-download.
type Options struct {
	Parallelism int
	Output      string
	URL         *url.URL
}

// Parse parses args and returns Options.
func Parse(args ...string) (*Options, error) {
	flg := flag.NewFlagSet("parallel-download", flag.ExitOnError)

	parallelism := flg.Int("p", 8, "parallelism")
	output := flg.String("o", "", "output")

	flg.Parse(args)

	u, err := url.ParseRequestURI(flg.Arg(0))
	if err != nil {
		return nil, err
	}

	if *output == "" {
		_, filename := path.Split(u.Path)

		// Inspired by the --default-page option of wget
		if filename == "" {
			filename = "index.html"
		}

		*output = filename
	}

	return &Options{
		Parallelism: *parallelism,
		Output:      *output,
		URL:         u,
	}, nil
}
