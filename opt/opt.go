/*
Package opt deals with CLI options.
*/
package opt

import (
	"flag"
	"net/url"
)

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

	return &Options{
		Parallelism: *parallelism,
		Output:      *output,
		URL:         u,
	}, nil
}
