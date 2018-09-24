/*
Package opt deals with CLI options.
*/
package opt

import (
	"errors"
	"flag"
	"net/url"
	"os"
	"path"
)

var errExist = errors.New("file already exists")

// Options has the options required for parallel-download.
type Options struct {
	Parallelism int
	DstFile     *os.File
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

	fp, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Options{
		Parallelism: *parallelism,
		DstFile:     fp,
		URL:         u,
	}, nil
}
